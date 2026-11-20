package worker

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	v1 "github.com/zrygan/media-upload-service/gen/videoupload/v1"
	"github.com/zrygan/media-upload-service/producer/network"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type srv struct {
	v1.UnimplementedVideoUploadServiceServer
	mu    sync.Mutex
	names []string
}

// GetUploadOffset reports that the consumer has none of the video yet.
func (s *srv) GetUploadOffset(context.Context, *v1.GetUploadOffsetRequest) (*v1.GetUploadOffsetResponse, error) {
	return &v1.GetUploadOffsetResponse{}, nil
}

func (s *srv) UploadVideo(st grpc.ClientStreamingServer[v1.UploadVideoRequest, v1.UploadVideoResponse]) error {
	first, err := st.Recv()
	if err != nil {
		return err
	}
	n := first.GetFileInfo().GetFileName()
	s.mu.Lock()
	s.names = append(s.names, n)
	s.mu.Unlock()
	if strings.Contains(n, "fail") {
		return status.Error(codes.Internal, "boom")
	}
	var res v1.VideoUploadStatus = v1.VideoUploadStatus_VIDEO_UPLOAD_ACCEPTED
	if strings.Contains(n, "drop") {
		res = v1.VideoUploadStatus_VIDEO_UPLOAD_DROPPED_QUEUE_FULL
	} else if strings.Contains(n, "dup") {
		res = v1.VideoUploadStatus_VIDEO_UPLOAD_DROPPED_DUPLICATE
	}
	if strings.Contains(n, "slow") {
		<-st.Context().Done()
		return st.Context().Err()
	}
	for {
		_, err := st.Recv()
		if errors.Is(err, io.EOF) {
			return st.SendAndClose(&v1.UploadVideoResponse{Status: res})
		}
		if err != nil {
			return err
		}
	}
}

func setup(t *testing.T) (*network.Client, *srv) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	g := grpc.NewServer()
	s := &srv{}
	v1.RegisterVideoUploadServiceServer(g, s)
	go g.Serve(l)
	t.Cleanup(g.Stop)
	c, err := network.New(l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c, s
}

// clip generates a short real video with ffmpeg, since uploads hash the
// video before sending it.
func clip(t *testing.T) []byte {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	p := filepath.Join(t.TempDir(), "clip.mp4")
	out, err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi",
		"-i", "testsrc=duration=1:size=320x240:rate=30", "-pix_fmt", "yuv420p", p).CombinedOutput()
	if err != nil {
		t.Fatalf("ffmpeg: %v: %s", err, out)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// folder makes a temp folder holding a copy of the same clip under
// each of names.
func folder(t *testing.T, names ...string) string {
	t.Helper()
	data := clip(t)
	d := t.TempDir()
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(d, n), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return d
}

func TestCountsContinueAndExit(t *testing.T) {
	c, s := setup(t)
	a := folder(t, "a-ok.mp4", "b-fail.mp4", "c-ok.mp4", "d-drop.mp4", "e-dup.mp4")
	os.Mkdir(filepath.Join(a, "subdir"), 0o755)
	b := folder(t, "z-ok.mp4")
	pws := MakeWorkers([]string{a, b})
	done := make(chan struct{})
	go func() { RunThreads(context.Background(), pws, c); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("RunThreads hung")
	}
	r := pws[0].rs
	t.Logf("worker0=%+v worker1=%+v names=%v", r, pws[1].rs, s.names)
	if r != (RunSummary{5, 2, 1, 1, 1}) {
		t.Fatalf("worker0 %+v", r)
	}
	if pws[1].rs != (RunSummary{1, 1, 0, 0, 0}) {
		t.Fatalf("worker1 %+v", pws[1].rs)
	}
	for _, n := range s.names {
		if n == "subdir" {
			t.Fatal("subdir uploaded")
		}
	}
}

func TestCancelStopsFolder(t *testing.T) {
	c, s := setup(t)
	a := folder(t, "a-slow.mp4", "b-ok.mp4", "c-ok.mp4")
	pws := MakeWorkers([]string{a})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { RunThreads(ctx, pws, c); close(done) }()
	time.Sleep(300 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("cancel did not stop RunThreads")
	}
	t.Logf("%+v names=%v", pws[0].rs, s.names)
	if pws[0].rs != (RunSummary{1, 0, 0, 0, 1}) {
		t.Fatalf("%+v", pws[0].rs)
	}
}

func TestMissingFolder(t *testing.T) {
	c, _ := setup(t)
	pws := MakeWorkers([]string{"/nope"})
	RunThreads(context.Background(), pws, c)
}
