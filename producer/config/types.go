package config

import (
	"fmt"
	"net"

	"github.com/zrygan/media-upload-service/util"
)

// ProducerConfig holds the configurations for a producer.
// This also defines the JSON schema via struct tags which are used
// in ReadConfigFromJSON.
type ProducerConfig struct {
	// Number of producer workers.
	ThreadCount int `json:"thread_count"`
	// The folders that contain videos that the user wants to upload.
	FolderList []string `json:"folder_list"`
	// The address of the consumer or the media upload service.
	ConsumerAddress string `json:"consumer_address"`
}

// Validate ensures that the config contains valid parameters. This
// checks the following:
//
//  1. ThreadCount is positive.
//  2. ThreadCount is equal to the number of folders.
//  3. Each folder in FolderList exists, is a directory, and can be
//     read.
//  4. ConsumerAddress is in the form host:port, with a host and a
//     valid port.
//
// This returns nil if all conditions are met; otherwise, it returns
// an error describing the first one that failed.
//
// Note that condition (4) only checks the FORMAT of ConsumerAddress,
// it does not check if the address has a consumer listening to it.
func (pc ProducerConfig) Validate() error {
	if pc.ThreadCount <= 0 {
		return fmt.Errorf("thread_count must be positive, got %d", pc.ThreadCount)
	}

	if len(pc.FolderList) != pc.ThreadCount {
		return fmt.Errorf("thread_count is %d but got %d folders", pc.ThreadCount, len(pc.FolderList))
	}

	for _, folder := range pc.FolderList {
		if err := util.CheckDirReadable(folder); err != nil {
			return err
		}
	}

	if err := util.CheckHostPort(pc.ConsumerAddress); err != nil {
		return fmt.Errorf("consumer_address: %w", err)
	}

	if host, _, _ := net.SplitHostPort(pc.ConsumerAddress); host == "" {
		return fmt.Errorf("consumer_address %q must have a host", pc.ConsumerAddress)
	}

	return nil
}
