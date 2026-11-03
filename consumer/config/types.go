package config

import (
	"fmt"

	"github.com/zrygan/media-upload-service/util"
)

type ConsumerConfig struct {
	// The maximum size or capacity of the upload queue.
	QueueMaxSize int `json:"queue_max_size"`
	// The number of workers the consumer has.
	ThreadCount int `json:"thread_count"`
	// The address the consumer listens to.
	ListenAddress string `json:"listen_address"`
	// The output or destination folder, the uploaded files go here.
	MediaFolder string `json:"media_folder"`
	// The staging folder, files in the middle of uploading go here
	// temporarily.
	StagingFolder string `json:"staging_folder"`
}

// Validate ensures that the config contains valid parameters. This
// checks the following:
//
//  1. QueueMaxSize and ThreadCount is positive.
//  2. ListenAddress is not empty.
//  3. MediaFolder and StagingFolder are both Readable and Writable.
//
// This returns nil if all conditions are met; otherwise, it returns
// an error describing the first one that failed.
//
// Note that condition (2) only checks if ConsumerAddress is EMPTY,
// it does not check for validity (if the address is valid and if the
// address has a consumer listening to it).
func (cc ConsumerConfig) Validate() error {
	if cc.QueueMaxSize < 0 {
		return fmt.Errorf("queue_max_size must be positive, got %d", cc.QueueMaxSize)
	}

	if cc.ThreadCount < 0 {
		return fmt.Errorf("thread_count must be positive, got %d", cc.ThreadCount)
	}

	if cc.ListenAddress == "" {
		return fmt.Errorf("consumer_address must not be empty")
	}

	if err := util.CheckDirReadable(cc.MediaFolder); err != nil {
		return err
	}

	if err := util.CheckDirReadable(cc.StagingFolder); err != nil {
		return err
	}

	if err := util.CheckDirWritable(cc.MediaFolder); err != nil {
		return err
	}

	if err := util.CheckDirWritable(cc.StagingFolder); err != nil {
		return err
	}

	return nil
}
