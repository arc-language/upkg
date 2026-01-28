// pkg/core/config.go
package core

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds upkg configuration
type Config struct {
	DefaultBackend string                 `yaml:"default_backend"`
	InstallPath    string                 `yaml:"install_path"`
	Debug          bool                   `yaml:"debug"`
	Backends       map[string]interface{} `yaml:"backends"`
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		DefaultBackend: "", // Auto-detect
		InstallPath:    getDefaultInstallPath(),
		Debug:          false,
		Backends:       make(map[string]interface{}),
	}
}

// LoadConfig loads configuration from file
func LoadConfig(path string) (*Config, error) {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return DefaultConfig(), nil
		}
		path = filepath.Join(home, ".config", "upkg", "config.yaml")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	return &cfg, nil
}

// SaveConfig saves configuration to file
func SaveConfig(cfg *Config, path string) error {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		path = filepath.Join(home, ".config", "upkg", "config.yaml")
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}

func getDefaultInstallPath() string {
	if path := os.Getenv("UPKG_INSTALL_PATH"); path != "" {
		return path
	}
	
	home, err := os.UserHomeDir()
	if err != nil {
		return "/usr/local"
	}
	
	return filepath.Join(home, ".upkg")
}