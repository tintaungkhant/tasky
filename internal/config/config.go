// Package config loads, saves, and manages tasky's YAML configuration.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Root is a configured top-level directory containing one or more repos.
type Root struct {
	Name           string `yaml:"name"`
	Path           string `yaml:"path"`
	TasksDir       string `yaml:"tasks_dir"`
	BranchTemplate string `yaml:"branch_template"`
}

// Config is tasky's persisted configuration.
type Config struct {
	Roots []Root `yaml:"roots"`
}

// ErrNoMatchingRoot is returned by MatchRoot when no configured Root contains the given path.
var ErrNoMatchingRoot = errors.New("no configured root contains this directory; run `tasky setup`")

// Path returns the path to tasky's config file, honoring $TASKY_CONFIG for tests.
func Path() (string, error) {
	if p := os.Getenv("TASKY_CONFIG"); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".config", "tasky", "config.yaml"), nil
}

// Load reads the config file. A missing file yields an empty Config, not an error.
func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}

// Save writes the config file, creating its parent directory if needed.
func (c *Config) Save() error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// AddRoot appends a Root to the config.
func (c *Config) AddRoot(r Root) {
	c.Roots = append(c.Roots, r)
}

// RemoveRoot removes the Root with the given name, if present.
func (c *Config) RemoveRoot(name string) bool {
	for i, r := range c.Roots {
		if r.Name == name {
			c.Roots = append(c.Roots[:i], c.Roots[i+1:]...)
			return true
		}
	}
	return false
}

// FindRoot returns the Root with the given name, if present.
func (c *Config) FindRoot(name string) (*Root, bool) {
	for i := range c.Roots {
		if c.Roots[i].Name == name {
			return &c.Roots[i], true
		}
	}
	return nil, false
}

// MatchRoot finds the configured Root containing dir, by longest path-prefix match.
func (c *Config) MatchRoot(dir string) (*Root, error) {
	var best *Root
	bestLen := -1
	for i := range c.Roots {
		rootPath := filepath.Clean(c.Roots[i].Path)
		cleanDir := filepath.Clean(dir)
		if cleanDir == rootPath || strings.HasPrefix(cleanDir, rootPath+string(filepath.Separator)) {
			if len(rootPath) > bestLen {
				best = &c.Roots[i]
				bestLen = len(rootPath)
			}
		}
	}
	if best == nil {
		return nil, ErrNoMatchingRoot
	}
	return best, nil
}
