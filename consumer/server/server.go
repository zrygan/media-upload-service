package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/zrygan/media-upload-service/consumer/queue"
	videouploadv1 "github.com/zrygan/media-upload-service/gen/videoupload/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// VideoServer is the consumer's implementation of the gRPC
// VideoUploadService. gRPC runs each call on its own goroutine, so
// many UploadVideo calls can run at the same time.
//
// The fields are only set in New, so the calls can read them without
// a lock. The queue is the only thing the calls share, and it is safe
// for concurrent use.
type VideoServer struct {
	q             *queue.Queue
	stagingFolder string
	// Required by gRPC-Go. This returns codes.Unimplemented for any RPC
	// that VideoServer does not implement.
	videouploadv1.UnimplementedVideoUploadServiceServer
}

// New returns a VideoServer that adds uploads to queue and stages them
// in the folder path.
//
// NOTE: path must be on the same filesystem as the media folder, since
// the workers move files out of it with os.Rename.
func New(queue *queue.Queue, path string) *VideoServer {

	return &VideoServer{q: queue, stagingFolder: path}
}

// UploadVideo receives one video from a producer and adds it to the
// queue. The first message must be the file info, and the rest are
// the video's data chunks (see UploadVideoRequest in the proto).
//
// This does the following:
//
//  1. Reads the file info.
//  2. Takes a queue permit. If the queue is full, this replies
//     VIDEO_UPLOAD_DROPPED_QUEUE_FULL before receiving any data.
//  3. Writes the chunks into a temp file in the staging folder.
//  4. Checks that the bytes received match the file size.
//  5. Adds the job to the queue and replies VIDEO_UPLOAD_ACCEPTED.
//
// A full queue is not an error, it is returned as the status. The
// error is only for broken uploads, like a stream error, a disk error,
// or a size mismatch. If this fails, the temp file is removed and the
// permit is given back.
func (s *VideoServer) UploadVideo(stream grpc.ClientStreamingServer[videouploadv1.UploadVideoRequest, videouploadv1.UploadVideoResponse]) error {

	first, err := stream.Recv()

	if err != nil {
		return err
	}

	info := first.GetFileInfo()

	if info == nil {
		return status.Error(codes.InvalidArgument, "First message must be FileInfo")
	}

	if s.q.TryReserve() == false {
		return stream.SendAndClose(&videouploadv1.UploadVideoResponse{Status: videouploadv1.VideoUploadStatus_VIDEO_UPLOAD_DROPPED_QUEUE_FULL})
	}

	// CreateTemp replaces the * with a random string, so two uploads
	// never share a staging file
	ext := filepath.Ext(info.FileName) // ".MP4"
	ext = strings.ToLower(ext)

	f, err := os.CreateTemp(s.stagingFolder, StagingPrefix+"*"+ext)

	if err != nil {
		s.q.UnReserve()
		return err
	}

	// while e is true, any return cleans up the temp file and the
	// permit. once the job is in the queue, both belong to a worker
	e := true
	defer func() {
		if e != false {
			f.Close()
			os.Remove(f.Name())
			s.q.UnReserve()
		}
	}()

	j := queue.Job{
		FileName:    info.FileName,
		FileSize:    info.FileSize,
		ProducerID:  info.ProducerId,
		Hash:        info.Hash,
		Signature:   info.Signature,
		StagingPath: f.Name(),
	}

	// write the chunks until the producer closes the stream (io.EOF)
	chunkSize := int64(0)
	for {
		data, err := stream.Recv()

		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return err
		}

		n, err := f.Write(data.GetData())

		if err != nil {
			return err
		}

		chunkSize += int64(n)

	}

	// Close can report a write error that Write did not
	err = f.Close()

	if err != nil {
		return err
	}

	// the producer stopped early or sent the wrong size
	if chunkSize != info.FileSize {
		return status.Errorf(codes.InvalidArgument,
			"size mismatch: expected %d bytes, received %d", info.FileSize, chunkSize)
	}

	// Holding a permit guarantees a free slot in the job channel, so
	// this only fails if the permit invariant is broken (for example a
	// double UnReserve). Treat it as a server bug, not a full queue.
	// e is still true here, so the deferred cleanup removes the staged
	// file and releases the permit.
	if s.q.TryEnqueue(j) == false {
		slog.Error("invariant broken: enqueue failed while holding a permit",
			"component", "server", "file", info.FileName, "producer_id", info.ProducerId,
			"reserved", s.q.CountReserve(), "pending", s.q.Pending(), "max", s.q.Size())
		return status.Error(codes.Internal, "internal error: could not enqueue upload")
	}
	e = false

	return stream.SendAndClose(&videouploadv1.UploadVideoResponse{Status: videouploadv1.VideoUploadStatus_VIDEO_UPLOAD_ACCEPTED})

}

// GetQueueStatus returns the maximum size of the queue and the number
// of slots in use. A slot is in use from the start of an upload until
// a worker takes its job, so CurrSize also counts uploads that are
// still arriving.
//
// NOTE: This is only a snapshot. A producer that sees a free slot can
// still get VIDEO_UPLOAD_DROPPED_QUEUE_FULL from UploadVideo.
func (s *VideoServer) GetQueueStatus(ctx context.Context, req *videouploadv1.GetQueueStatusRequest) (*videouploadv1.GetQueueStatusResponse, error) {
	return &videouploadv1.GetQueueStatusResponse{MaxSize: int32(s.q.Size()), CurrSize: int32(s.q.CountReserve())}, nil
}
