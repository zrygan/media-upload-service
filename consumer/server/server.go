package server

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/zrygan/media-upload-service/consumer/queue"
	videouploadv1 "github.com/zrygan/media-upload-service/gen/videoupload/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type VideoServer struct {
	q             *queue.Queue
	stagingFolder string
	videouploadv1.UnimplementedVideoUploadServiceServer
}

func New(queue *queue.Queue, path string) *VideoServer {

	return &VideoServer{q: queue, stagingFolder: path}
}

func (s *VideoServer) UploadVideo(stream grpc.ClientStreamingServer[videouploadv1.UploadVideoRequest, videouploadv1.UploadVideoResponse]) error {

	first, err := stream.Recv()

	if err != nil {
		return err
	}

	info := first.GetFileInfo()

	if info == nil {
		return status.Error(codes.InvalidArgument, "First message must be FileInfo")
	}

	ext := filepath.Ext(info.FileName) // ".MP4"
	ext = strings.ToLower(ext)

	f, err := os.CreateTemp(s.stagingFolder, "upload-*"+ext)

	if err != nil {
		return err
	}

	e := true
	defer func() {
		if e != false {
			f.Close()
			os.Remove(f.Name())
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
	err = f.Close()

	if err != nil {
		return err
	}

	if chunkSize != info.FileSize {
		return status.Errorf(codes.InvalidArgument,
			"size mismatch: expected %d bytes, received %d", info.FileSize, chunkSize)
	}

	if s.q.TryEnqueue(j) == false {
		return stream.SendAndClose(&videouploadv1.UploadVideoResponse{Status: videouploadv1.VideoUploadStatus_VIDEO_UPLOAD_DROPPED_QUEUE_FULL})
	}
	e = false

	return stream.SendAndClose(&videouploadv1.UploadVideoResponse{Status: videouploadv1.VideoUploadStatus_VIDEO_UPLOAD_ACCEPTED})

}

func (s *VideoServer) GetQueueStatus(ctx context.Context, req *videouploadv1.GetQueueStatusRequest) (*videouploadv1.GetQueueStatusResponse, error) {
	return &videouploadv1.GetQueueStatusResponse{MaxSize: int32(s.q.Size()), CurrSize: int32(s.q.Pending())}, nil
}
