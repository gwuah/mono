package mono

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Scripts Scripts `yaml:"scripts"`
}

type Scripts struct {
	Init    string `yaml:"init"`
	Setup   string `yaml:"setup"`
	Run     string `yaml:"run"`
	Destroy string `yaml:"destroy"`
}

func LoadConfig(dir string) (*Config, error) {
	path := filepath.Join(dir, "mono.yml")

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read mono.yml: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid mono.yml: %w", err)
	}

	return &cfg, nil
}
