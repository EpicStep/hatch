package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds the hatch project settings loaded from .hatch.yaml.
type Config struct {
	Namespace string `yaml:"namespace"`
	Kind      string `yaml:"kind"`
	Workload  string `yaml:"workload"`
	Container string `yaml:"container"`
	Image     string `yaml:"image"`
}

// Load reads the config from path. If path is empty, it defaults to ".hatch.yaml".
// Returns an empty Config when the file does not exist.
func Load(path string) (*Config, error) {
	if path == "" {
		path = ".hatch.yaml"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}

		return nil, fmt.Errorf("os.ReadFile(%s): %w", path, err)
	}

	var cfg Config
	if err = yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("yaml.Unmarshal(%s): %w", path, err)
	}

	return &cfg, nil
}
