package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// ReadConfigFromJSON returns a ProducerConfig whose values come from
// a JSON file. The schema of the JSON is handled via the struct tags
// in ProducerConfig.
//
// This loads the JSON file and validates the config from it. If
// either step fails, this returns a nil config and an error, so a
// non-nil config is always valid.
func ReadConfigFromJSON(filename string) (*ProducerConfig, error) {
	file, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("cannot open config %s: %w", filename, err)
	}

	var config ProducerConfig
	if err := json.Unmarshal(file, &config); err != nil {
		return nil, fmt.Errorf("cannot parse config %s: %w", filename, err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config %s: %w", filename, err)
	}

	return &config, nil
}
