package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// File / directory names within a config directory.
const (
	FileName   = "config.yaml"
	ImagesDir  = "images"
	BackupsDir = "backups"
	DBFileName = "upmonitor.db"
)

// YAMLPath returns the path to config.yaml inside dir.
func YAMLPath(dir string) string { return filepath.Join(dir, FileName) }

// ImagesPath returns the images directory inside dir.
func ImagesPath(dir string) string { return filepath.Join(dir, ImagesDir) }

// BackupsPath returns the backups directory inside dir.
func BackupsPath(dir string) string { return filepath.Join(dir, BackupsDir) }

// DBPath returns the SQLite database path inside dir.
func DBPath(dir string) string { return filepath.Join(dir, DBFileName) }

// EnsureDir creates the config directory and its images/ subdirectory.
func EnsureDir(dir string) error {
	if err := os.MkdirAll(ImagesPath(dir), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	return nil
}

// Load reads and validates config.yaml from dir. A missing file yields a
// default config (first-run friendly); a present-but-invalid file is an error.
func Load(dir string) (*Config, error) {
	data, err := os.ReadFile(YAMLPath(dir))
	if os.IsNotExist(err) {
		return Default(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	return Parse(data)
}

// Parse unmarshals and validates raw YAML bytes.
func Parse(data []byte) (*Config, error) {
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

// Marshal renders the config as YAML bytes.
func Marshal(c *Config) ([]byte, error) {
	return yaml.Marshal(c)
}

// Save validates and atomically writes the config to dir (temp file + rename),
// so a crash mid-write never corrupts config.yaml.
func Save(dir string, c *Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if err := EnsureDir(dir); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpName, YAMLPath(dir)); err != nil {
		return fmt.Errorf("rename config: %w", err)
	}
	return nil
}
