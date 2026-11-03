package config

import (
	"fmt"

	"github.com/zrygan/media-upload-service/util"
)

// ConsumerConfig holds the configurations for a consumer.
// This also defines the JSON schema via struct tags which are used
// in ReadConfigFromJSON.
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
//  2. ListenAddress is in the form host:port with a valid port. The
//     host can be empty (like :50051).
//  3. MediaFolder and StagingFolder are both Readable and Writable.
//
// This returns nil if all conditions are met; otherwise, it returns
// an error describing the first one that failed.
//
// Note that condition (2) only checks the FORMAT of ListenAddress, it
// does not check if the port is free.
func (cc ConsumerConfig) Validate() error {
	if cc.QueueMaxSize <= 0 {
		return fmt.Errorf("queue_max_size must be positive, got %d", cc.QueueMaxSize)
	}

	if cc.ThreadCount <= 0 {
		return fmt.Errorf("thread_count must be positive, got %d", cc.ThreadCount)
	}

	if err := util.CheckHostPort(cc.ListenAddress); err != nil {
		return fmt.Errorf("listen_address: %w", err)
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
