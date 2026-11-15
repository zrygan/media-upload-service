package server

import (
	"context"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"os"

	"github.com/zrygan/media-upload-service/consumer/inflight"
	"github.com/zrygan/media-upload-service/consumer/queue"
	videouploadv1 "github.com/zrygan/media-upload-service/gen/videoupload/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// VideoServer implements the gRPC VideoUploadService. gRPC runs each
// call on its own goroutine. The fields never change after New, so
// reading them needs no lock, and q and h are safe for concurrent use.
type VideoServer struct {
	q             *queue.Queue
	h             *inflight.Hashbrowns
	stagingFolder string
	// Answers codes.Unimplemented for RPCs not written here.
	videouploadv1.UnimplementedVideoUploadServiceServer
}

// New returns a VideoServer. path must be on the same filesystem as the
// media folder, because workers move files out of it with os.Rename.
func New(queue *queue.Queue, path string, hashbrowns *inflight.Hashbrowns) *VideoServer {

	return &VideoServer{q: queue, stagingFolder: path, h: hashbrowns}
}

// UploadVideo receives one video: a file info message, then data chunks.
//
//  1. Check the hash and claim it in h. If another upload has it, reply
//     ACCEPTED (already queued) or codes.Aborted (still uploading).
//  2. Take a queue permit, or reply DROPPED_QUEUE_FULL.
//  3. Open the hash's .part file and line it up with info.Offset.
//  4. Append the chunks and check the total size.
//  5. Mark the hash Queued, queue the job, reply ACCEPTED.
//
// On failure the hash and the permit are released. The .part file is
// kept so the producer can resume, unless its size is wrong.
func (s *VideoServer) UploadVideo(stream grpc.ClientStreamingServer[videouploadv1.UploadVideoRequest, videouploadv1.UploadVideoResponse]) error {

	first, err := stream.Recv()

	if err != nil {
		return err
	}

	info := first.GetFileInfo()

	if info == nil {
		return status.Error(codes.InvalidArgument, "First message must be FileInfo")
	}

	tempPath, err := tempFile(s.stagingFolder, info.Hash)
	if err != nil {
		return status.Error(codes.InvalidArgument, "Recived invalid byte size")
	}

	hash := hex.EncodeToString(info.Hash)
	hst, started := s.h.TryUpload(hash)
	if !started && hst == inflight.Queued {
		// an earlier attempt already queued it; the producer missed the reply
		return stream.SendAndClose(&videouploadv1.UploadVideoResponse{Status: videouploadv1.VideoUploadStatus_VIDEO_UPLOAD_ACCEPTED})
	}
	if !started {
		return status.Error(codes.Aborted, "Upload already in progress")
	}

	// While e is true, any return runs the cleanup in the two defers
	// (hash, then permit and file). Once the job is queued (e = false),
	// all three belong to a worker.
	e := true
	defer func() {
		if e == true {
			s.h.Drop(hash)
		}
	}()

	if s.q.TryReserve() == false {
		return stream.SendAndClose(&videouploadv1.UploadVideoResponse{Status: videouploadv1.VideoUploadStatus_VIDEO_UPLOAD_DROPPED_QUEUE_FULL})
	}

	// no O_TRUNC: a resume keeps the bytes already there
	f, err := os.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY, 0o644)

	// keep = false makes the cleanup delete the file (its bytes are wrong)
	keep := true
	defer func() {
		if e {
			f.Close()
			s.q.UnReserve()
			if !keep {
				os.Remove(f.Name())
			}
		}
	}()

	if err != nil {
		return err
	}

	st, err := f.Stat()

	if err != nil {
		return err
	}

	// Line the file up with info.Offset. Shorter means the producer's
	// offset is out of date (it asks again and retries). Longer means
	// cut off the bytes it is about to resend. tempOffset ends up as
	// the file's total size.
	tempOffset := st.Size()

	switch {
	case tempOffset < info.Offset:
		return status.Errorf(codes.Aborted,
			"offset mismatch: expected %d bytes, received %d", tempOffset, info.Offset)
	case tempOffset > info.Offset:
		err = f.Truncate(info.Offset)
		if err != nil {
			return err
		}
		tempOffset = info.Offset
	}

	_, err = f.Seek(info.Offset, io.SeekStart)
	if err != nil {
		return err
	}

	j := queue.Job{
		FileName:    info.FileName,
		FileSize:    info.FileSize,
		ProducerID:  info.ProducerId,
		Hash:        info.Hash,
		Signature:   info.Signature,
		StagingPath: f.Name(),
	}

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

		tempOffset += int64(n)

	}

	// Close can report a write error that Write did not
	err = f.Close()

	if err != nil {
		return err
	}

	// wrong size: the bytes can't be trusted, so delete instead of keep
	if tempOffset != info.FileSize {
		keep = false
		return status.Errorf(codes.InvalidArgument,
			"size mismatch: expected %d bytes, received %d", info.FileSize, tempOffset)
	}

	// before TryEnqueue, not after (see HashQueued)
	s.h.HashQueued(hash)

	// The permit guarantees room, so this only fails on a bug (a broken
	// permit count). e is still true, so the hash and permit are
	// released. The file passed the size check, so it is kept.
	if s.q.TryEnqueue(j) == false {
		slog.Error("invariant broken: enqueue failed while holding a permit",
			"component", "server", "file", info.FileName, "producer_id", info.ProducerId,
			"reserved", s.q.CountReserve(), "pending", s.q.Pending(), "max", s.q.Size())
		return status.Error(codes.Internal, "internal error: could not enqueue upload")
	}
	e = false

	return stream.SendAndClose(&videouploadv1.UploadVideoResponse{Status: videouploadv1.VideoUploadStatus_VIDEO_UPLOAD_ACCEPTED})

}

// GetQueueStatus returns the queue's capacity and the slots in use,
// which includes uploads still arriving. It is only a snapshot: a free
// slot can be gone by the next UploadVideo.
func (s *VideoServer) GetQueueStatus(ctx context.Context, req *videouploadv1.GetQueueStatusRequest) (*videouploadv1.GetQueueStatusResponse, error) {
	return &videouploadv1.GetQueueStatusResponse{MaxSize: int32(s.q.Size()), CurrSize: int32(s.q.CountReserve())}, nil
}

// GetUploadOffset returns how many bytes of the video (found by hash)
// are already staged, so the producer can resume. 0 means none. A bad
// hash returns codes.InvalidArgument (retrying won't help), and an
// unreadable file returns codes.Internal.
func (s *VideoServer) GetUploadOffset(ctx context.Context, req *videouploadv1.GetUploadOffsetRequest) (*videouploadv1.GetUploadOffsetResponse, error) {

	path, err := tempFile(s.stagingFolder, req.Hash)

	if err != nil {
		slog.Warn("bad hash in GetUploadOffset",
			"component", "server", "hash_len", len(req.Hash), "error", err)
		return nil, status.Error(codes.InvalidArgument, "recived non-32 bytes")
	}

	st, err := os.Stat(path)

	if errors.Is(err, fs.ErrNotExist) {
		return &videouploadv1.GetUploadOffsetResponse{Offset: 0}, nil
	}

	if err != nil {
		slog.Error("could not read staging file size",
			"component", "server", "path", path, "error", err)
		return nil, status.Error(codes.Internal, "cannot read staging file size")
	}

	return &videouploadv1.GetUploadOffsetResponse{Offset: st.Size()}, nil
}
