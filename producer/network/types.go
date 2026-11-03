package network

import (
	videouploadv1 "github.com/zrygan/media-upload-service/gen/videoupload/v1"
	"google.golang.org/grpc"
)

// Client holds the connection to the consumer and the generated gRPC
// client that uses it. A Client is safe to share across goroutines.
type Client struct {
	conn *grpc.ClientConn
	// The generated client for the consumer's VideoUploadService.
	Service videouploadv1.VideoUploadServiceClient
}
