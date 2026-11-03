package upload

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	videouploadv1 "github.com/zrygan/media-upload-service/gen/videoupload/v1"
	"github.com/zrygan/media-upload-service/producer/network"
	videohash "github.com/zrygan/media-upload-service/producer/video_hash"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// The size of each data message. This must stay below gRPC's 4 MiB
	// message limit, which also counts the message's own overhead.
	chunkSize = 1 << 20
	// The maximum time for one upload.
	uploadTimeout = 10 * time.Minute
	// Status shorthands --- this is for readability purposes.
	statusUnknown = videouploadv1.VideoUploadStatus_VIDEO_UPLOAD_STATUS_UNSPECIFIED
	statusError   = videouploadv1.VideoUploadStatus_VIDEO_UPLOAD_ERROR
	// The delay per upload retry.
	retryDelay = 1 * time.Second
	// The maximum number of retries before quitting the upload
	retryMaxAttempts = 15
	// The multiplier applied to retryDelay after each retry attempt.
	retryDelayMultiplier = 2
	// The maximum delay.
	retryMaxDelay = 30 * time.Second
)

// isRetryable returns true if err means the consumer could not be reached.
func isRetryable(err error) bool {
	return status.Code(err) == codes.Unavailable
}

// File uploads the file at path to the consumer and returns the status
// the consumer responded with.
//
// A dropped upload is not an error, this returns the status (like
// VIDEO_UPLOAD_DROPPED_QUEUE_FULL) and a nil error. The error is only
// for uploads that failed, like if the file cannot be read or if the
// consumer cannot be reached.
func File(ctx context.Context, client *network.Client, producerID int, path string) (videouploadv1.VideoUploadStatus, error) {
	file, err := os.Open(path)
	if err != nil {
		return statusUnknown, fmt.Errorf("upload %s: %w", path, err)
	}
	defer file.Close()

	stats, err := file.Stat()
	if err != nil {
		return statusUnknown, fmt.Errorf("upload %s: %w", path, err)
	}

	hash, err := videohash.SHA256(file)
	if err != nil {
		return statusUnknown, fmt.Errorf("upload: %w", err)
	}
	signature, err := videohash.VideoSignature(ctx, file)
	if err != nil {
		return statusUnknown, fmt.Errorf("upload: %w", err)
	}

	info := &videouploadv1.VideoUploadInfo{
		FileName:   filepath.Base(path),
		FileSize:   stats.Size(),
		ProducerId: int32(producerID),
		Hash:       hash,
		Signature:  signature,
	}

	slog.Info("upload started",
		"component", "upload",
		"producer_id", producerID,
		"file", info.GetFileName(),
		"size_bytes", info.GetFileSize(),
	)

	job := uploadJob{client: client, info: info, file: file}

	status, err := job.uploadOrTryAgain(ctx)
	if err != nil {
		return statusError, err
	}

	return status, nil
}

// uploadJob holds what stays the same across the attempts of one upload.
type uploadJob struct {
	client *network.Client
	info   *videouploadv1.VideoUploadInfo
	file   *os.File
}

// uploadOrTryAgain keeps retrying the upload (given that it has not
// met the maximum number of retries, see retryMaxAttempts). If we
// have retried the maximum number of times, an error occurs and any
// retry will not resolve, or the context runs out, we fail the upload.
func (job uploadJob) uploadOrTryAgain(ctx context.Context) (videouploadv1.VideoUploadStatus, error) {
	currDelay := retryDelay
	var lastErr error

	for attempt := range retryMaxAttempts {
		status, err := job.attemptUpload(ctx)
		if err == nil {
			return status, nil
		} else if !isRetryable(err) {
			return statusError, fmt.Errorf("uploadOrTryAgain %s: %w", job.file.Name(), err)
		}

		lastErr = err

		if attempt == retryMaxAttempts-1 {
			break
		}

		slog.Warn("upload attempt failed, retrying",
			"component", "upload",
			"producer_id", job.info.GetProducerId(),
			"file", job.info.GetFileName(),
			"attempt", attempt+1,
			"max_attempts", retryMaxAttempts,
			"retry_in", currDelay.String(),
			"error", err,
		)

		select {
		case <-time.After(currDelay):
		case <-ctx.Done():
			return statusUnknown, ctx.Err()
		}

		currDelay *= retryDelayMultiplier
		if currDelay > retryMaxDelay {
			currDelay = retryMaxDelay
		}
	}

	return statusError, fmt.Errorf("uploadOrTryAgain %s: already tried the maximum number of times (%d): %w", job.file.Name(), retryMaxAttempts, lastErr)
}

// attemptUpload makes one upload from the start of the file. This has
// its own uploadTimeout, so each attempt gets the full time.
func (job uploadJob) attemptUpload(ctx context.Context) (videouploadv1.VideoUploadStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, uploadTimeout)
	defer cancel()

	if _, err := job.file.Seek(0, io.SeekStart); err != nil {
		return statusUnknown, err
	}

	stream, err := job.client.Service.UploadVideo(ctx)
	if err != nil {
		return statusError, fmt.Errorf("attemptUpload %s: %w", job.file.Name(), err)
	}

	err = sendFile(stream, job.info, job.file)
	if err != nil && !errors.Is(err, io.EOF) {
		return statusError, fmt.Errorf("attemptUpload %s: %w", job.file.Name(), err)
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		return statusError, fmt.Errorf("attemptUpload %s: %w", job.file.Name(), err)
	}

	return resp.GetStatus(), nil
}

// sendFile sends the file's info as the first message, then the
// contents of r as data messages. This stops at the first error.
//
// After the metadata, the video (fragmented into chunks) is sent
// to the consumer. See sendChunks.
//
// NOTE: As discussed in the protoc file, the first upload request
// is the file metadata (this will be used to save the file name,
// etc.) and to check for duplicates before sending any bytes of data
// into the consumer.
func sendFile(
	stream grpc.ClientStreamingClient[videouploadv1.UploadVideoRequest, videouploadv1.UploadVideoResponse],
	info *videouploadv1.VideoUploadInfo,
	r io.Reader,
) error {
	err := stream.Send(&videouploadv1.UploadVideoRequest{
		Payload: &videouploadv1.UploadVideoRequest_FileInfo{FileInfo: info},
	})
	if err != nil {
		return err
	}

	return sendChunks(r, func(chunk []byte) error {
		return stream.Send(&videouploadv1.UploadVideoRequest{
			Payload: &videouploadv1.UploadVideoRequest_Data{Data: chunk},
		})
	})
}

// sendChunks reads the file until the end of file and passes each
// chunk to send. This returns nil when file is fully read; otherwise,
// it returns the first error from reading or from send.
//
// For more information on why we fragment the video, see the
// documentation of chunkSize.
func sendChunks(r io.Reader, send func([]byte) error) error {
	buf := make([]byte, chunkSize)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			if sendErr := send(buf[:n]); sendErr != nil {
				return sendErr
			}
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}
