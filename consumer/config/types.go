package config

import (
	"fmt"

	"github.com/zrygan/media-upload-service/util"
)

// ConsumerConfig is the consumer's settings. The struct tags are the
// JSON field names.
type ConsumerConfig struct {
	// The queue's capacity (q). Uploads that arrive when it is full are
	// dropped.
	QueueMaxSize int `json:"queue_max_size"`
	// The number of workers (c).
	ThreadCount int `json:"thread_count"`
	// The host:port to listen on. The host can be empty, as in ":9090".
	ListenAddress string `json:"listen_address"`
	// Where saved videos go.
	MediaFolder string `json:"media_folder"`
	// Where uploads stay until they are complete. It must be on the same
	// filesystem as MediaFolder, because workers move files with
	// os.Rename.
	StagingFolder string `json:"staging_folder"`
}

// Validate returns an error for the first invalid setting: q and c must
// be positive, ListenAddress must look like host:port (the port may
// still be taken), and both folders must be readable and writable.
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
