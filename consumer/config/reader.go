package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// ReadConfigFromJSON reads and validates the config in filename. A
// non-nil result is always valid.
func ReadConfigFromJSON(filename string) (*ConsumerConfig, error) {
	file, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("cannot open config %s: %w", filename, err)
	}

	var config ConsumerConfig
	if err := json.Unmarshal(file, &config); err != nil {
		return nil, fmt.Errorf("cannot parse config %s: %w", filename, err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config %s: %w", filename, err)
	}

	return &config, nil
}
