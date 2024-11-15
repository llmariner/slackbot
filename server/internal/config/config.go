package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the configuration.
type Config struct {
	LLMarinerBaseURL string `yaml:"llmarinerBaseUrl"`
	ModelID          string `yaml:"modelId"`
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.LLMarinerBaseURL == "" {
		return fmt.Errorf("llmarinerBaseUrl is required")
	}
	if c.ModelID == "" {
		return fmt.Errorf("modelId is required")
	}

	return nil
}

// Parse parses the configuration file at the given path, returning a new
// Config struct.
func Parse(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read: %s", err)
	}
	var config Config
	if err = yaml.Unmarshal(b, &config); err != nil {
		return nil, fmt.Errorf("config: unmarshal: %s", err)
	}
	return &config, nil
}
