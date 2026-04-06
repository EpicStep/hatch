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
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	return &cfg, nil
}

// ApplyDefaults fills zero-value fields with sensible defaults.
func (c *Config) ApplyDefaults() {
	if c.Namespace == "" {
		c.Namespace = "default"
	}
	if c.Kind == "" {
		c.Kind = "daemonset"
	}
	if c.Image == "" {
		c.Image = "ghcr.io/epicstep/hatch:latest"
	}
}
