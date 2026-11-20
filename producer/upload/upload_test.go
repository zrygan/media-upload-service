package upload

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	v1 "github.com/zrygan/media-upload-service/gen/videoupload/v1"
	"github.com/zrygan/media-upload-service/producer/network"
	"google.golang.org/grpc"
)

type srv struct {
	v1.UnimplementedVideoUploadServiceServer
	drop bool
	got  bytes.Buffer
	info *v1.VideoUploadInfo
}

// GetUploadOffset reports that the consumer has none of the video yet, so
// File sends it from the start.
func (s *srv) GetUploadOffset(context.Context, *v1.GetUploadOffsetRequest) (*v1.GetUploadOffsetResponse, error) {
	return &v1.GetUploadOffsetResponse{Offset: 0}, nil
}

func (s *srv) UploadVideo(st grpc.ClientStreamingServer[v1.UploadVideoRequest, v1.UploadVideoResponse]) error {
	first, err := st.Recv()
	if err != nil {
		return err
	}
	s.info = first.GetFileInfo()
	if s.drop {
		return st.SendAndClose(&v1.UploadVideoResponse{Status: v1.VideoUploadStatus_VIDEO_UPLOAD_DROPPED_QUEUE_FULL})
	}
	for {
		m, err := st.Recv()
		if errors.Is(err, io.EOF) {
			return st.SendAndClose(&v1.UploadVideoResponse{Status: v1.VideoUploadStatus_VIDEO_UPLOAD_ACCEPTED})
		}
		if err != nil {
			return err
		}
		s.got.Write(m.GetData())
	}
}

func setup(t *testing.T, s *srv) *network.Client {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	g := grpc.NewServer()
	v1.RegisterVideoUploadServiceServer(g, s)
	go g.Serve(l)
	t.Cleanup(g.Stop)
	c, err := network.New(l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

// write generates a real H.264 clip with ffmpeg, since File hashes the video
// before uploading. secs controls the length (and roughly the size).
func write(t *testing.T, secs int) (string, []byte) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	p := filepath.Join(t.TempDir(), "clip.mp4")
	cmd := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi",
		"-i", fmt.Sprintf("testsrc=duration=%d:size=640x360:rate=30", secs),
		"-pix_fmt", "yuv420p", p)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg: %v: %s", err, out)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return p, data
}

func TestAccepted(t *testing.T) {
	for _, secs := range []int{1, 3, 10} {
		s := &srv{}
		p, data := write(t, secs)
		size := len(data)
		st, err := File(context.Background(), setup(t, s), 7, p)
		if err != nil || st != v1.VideoUploadStatus_VIDEO_UPLOAD_ACCEPTED || !bytes.Equal(s.got.Bytes(), data) {
			t.Fatalf("size=%d st=%v err=%v got=%d", size, st, err, s.got.Len())
		}
		if s.info.GetFileName() != "clip.mp4" || s.info.GetFileSize() != int64(size) || s.info.GetProducerId() != 7 {
			t.Fatalf("info=%v", s.info)
		}
		t.Logf("size=%d ok name=%s", size, s.info.GetFileName())
	}
}

func TestDroppedEarly(t *testing.T) {
	p, _ := write(t, 10)
	st, err := File(context.Background(), setup(t, &srv{drop: true}), 1, p)
	t.Logf("st=%v err=%v", st, err)
	if err != nil || st != v1.VideoUploadStatus_VIDEO_UPLOAD_DROPPED_QUEUE_FULL {
		t.Fatal("expected dropped")
	}
}

func TestMissingFile(t *testing.T) {
	st, err := File(context.Background(), setup(t, &srv{}), 1, "/nope/x.mp4")
	t.Logf("st=%v err=%v", st, err)
	if err == nil || !errors.Is(err, os.ErrNotExist) || st != statusUnknown {
		t.Fatal("expected wrapped not-exist")
	}
}

func TestCancelled(t *testing.T) {
	p, _ := write(t, 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	st, err := File(ctx, setup(t, &srv{}), 1, p)
	t.Logf("st=%v err=%v", st, err)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUnreachable(t *testing.T) {
	c, _ := network.New("127.0.0.1:1")
	defer c.Close()
	p, _ := write(t, 1)
	// File retries an unreachable consumer with backoff, so bound it.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	st, err := File(ctx, c, 1, p)
	t.Logf("st=%v err=%v", st, err)
	if err == nil {
		t.Fatal("expected error")
	}
}
