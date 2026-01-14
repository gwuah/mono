package mono

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type ArtifactConfig struct {
	Name        string   `yaml:"name"`
	KeyFiles    []string `yaml:"key_files"`
	KeyCommands []string `yaml:"key_commands"`
	Paths       []string `yaml:"paths"`
}

type BuildConfig struct {
	Sccache   *bool            `yaml:"sccache"`
	Artifacts []ArtifactConfig `yaml:"artifacts"`
}

type Config struct {
	Scripts Scripts     `yaml:"scripts"`
	Build   BuildConfig `yaml:"build"`
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

func (c *Config) ApplyDefaults(envPath string) {
	if len(c.Build.Artifacts) == 0 {
		c.Build.Artifacts = detectArtifacts(envPath)
	}
}

func detectArtifacts(envPath string) []ArtifactConfig {
	var artifacts []ArtifactConfig

	if fileExists(filepath.Join(envPath, "Cargo.lock")) {
		artifacts = append(artifacts, ArtifactConfig{
			Name:        "cargo",
			KeyFiles:    []string{"Cargo.lock"},
			KeyCommands: []string{"rustc --version"},
			Paths:       []string{"target"},
		})
	}

	if fileExists(filepath.Join(envPath, "package-lock.json")) {
		artifacts = append(artifacts, ArtifactConfig{
			Name:        "npm",
			KeyFiles:    []string{"package-lock.json"},
			KeyCommands: []string{"node --version"},
			Paths:       []string{"node_modules"},
		})
	}

	if fileExists(filepath.Join(envPath, "yarn.lock")) {
		artifacts = append(artifacts, ArtifactConfig{
			Name:        "yarn",
			KeyFiles:    []string{"yarn.lock"},
			KeyCommands: []string{"node --version"},
			Paths:       []string{"node_modules"},
		})
	}

	if fileExists(filepath.Join(envPath, "pnpm-lock.yaml")) {
		artifacts = append(artifacts, ArtifactConfig{
			Name:        "pnpm",
			KeyFiles:    []string{"pnpm-lock.yaml"},
			KeyCommands: []string{"node --version"},
			Paths:       []string{"node_modules"},
		})
	}

	return artifacts
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
