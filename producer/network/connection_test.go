package network

import (
	"context"
	"testing"

	videouploadv1 "github.com/zrygan/media-upload-service/gen/videoupload/v1"
)

// TestCloseStopsRPCs checks that a Client cannot make calls after Close.
func TestCloseStopsRPCs(t *testing.T) {
	c, err := New("127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	_, err = c.Service.GetQueueStatus(context.Background(), &videouploadv1.GetQueueStatusRequest{})
	if err == nil {
		t.Fatal("expected error after Close")
	}
}
