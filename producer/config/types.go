package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// ProducerConfig holds the configurations for a producer.
// This also defines the JSON schema via struct tags which are used
// in ReadConfigFromJSON.
type ProducerConfig struct {
	ThreadCount     int      `json:"thread_count"`
	FolderList      []string `json:"folder_list"`
	ConsumerAddress string   `json:"consumer_address"`
}

// validate ensures that the config contains valid parameters. This
// checks the following:
//
//  1. ThreadCount is positive.
//  2. ThreadCount is equal to the number of folders.
//  3. Each folder in FolderList exists, is a directory, and can be
//     read.
//  4. ConsumerAddress is not empty.
//
// This returns nil if all conditions are met; otherwise, it returns
// an error describing the first one that failed.
func (pc ProducerConfig) validate() error {
	if pc.ThreadCount <= 0 {
		return fmt.Errorf("thread_count must be positive, got %d", pc.ThreadCount)
	}

	if len(pc.FolderList) != pc.ThreadCount {
		return fmt.Errorf("thread_count is %d but got %d folders", pc.ThreadCount, len(pc.FolderList))
	}

	for _, folder := range pc.FolderList {
		if _, err := os.ReadDir(folder); err != nil {
			return fmt.Errorf("cannot open folder %s: %w", folder, err)
		}
	}

	if pc.ConsumerAddress == "" {
		return fmt.Errorf("consumer_address must not be empty")
	}

	return nil
}

// ReadConfigFromJSON returns a ProducerConfig whose values come from
// a JSON file. The schema of the JSON is handled via the struct tags
// in ProducerConfig.
//
// This loads the JSON file and validates the config from the JSON
// file. This loads the JSON file and validates the config from it.
// If either step fails, this returns a nil config and an error,
// so a non-nil config is always valid.
func ReadConfigFromJSON(filename string) (*ProducerConfig, error) {
	file, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("cannot open config %s: %w", filename, err)
	}

	var config ProducerConfig
	if err := json.Unmarshal(file, &config); err != nil {
		return nil, fmt.Errorf("cannot parse config %s: %w", filename, err)
	}

	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("invalid config %s: %w", filename, err)
	}

	return &config, nil
}
