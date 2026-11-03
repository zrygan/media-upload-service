package network

import (
	"fmt"

	videouploadv1 "github.com/zrygan/media-upload-service/gen/videoupload/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// New returns a Client for the consumer at address. The caller must
// call Close on the returned Client when it is done with it.
//
// NOTE: This does not connect to the consumer. The connection is
// made on the first RPC, so an unreachable consumer is reported
// there. This DOES NOT check if the consumer is reachable or not!
func New(address string) (*Client, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("cannot create client for consumer %s: %w", address, err)
	}

	return &Client{
		conn:    conn,
		Service: videouploadv1.NewVideoUploadServiceClient(conn),
	}, nil
}

// Close closes the connection to the consumer.
func (c *Client) Close() error {
	return c.conn.Close()
}
