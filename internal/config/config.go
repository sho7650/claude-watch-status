package config

import (
	"os"
	"path/filepath"
)

// Config holds the application configuration
type Config struct {
	ProjectsDir string
	ServerPort  int
	HooksPort   int
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	return &Config{
		ProjectsDir: filepath.Join(homeDir, ".claude", "projects"),
		ServerPort:  10087,
		HooksPort:   10087,
	}
}

// NewConfig creates a new configuration with optional overrides
func NewConfig(opts ...Option) *Config {
	cfg := DefaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

// Option is a function that modifies the configuration
type Option func(*Config)

// WithProjectsDir sets the projects directory
func WithProjectsDir(dir string) Option {
	return func(c *Config) {
		if dir != "" {
			c.ProjectsDir = dir
		}
	}
}

// WithServerPort sets the server port
func WithServerPort(port int) Option {
	return func(c *Config) {
		if port > 0 {
			c.ServerPort = port
		}
	}
}

// GetProjectsDir returns the projects directory, checking env var first
func GetProjectsDir() string {
	if dir := os.Getenv("CLAUDE_PROJECTS_DIR"); dir != "" {
		return dir
	}
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".claude", "projects")
}

// GetClaudeDir returns the Claude configuration directory
func GetClaudeDir() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".claude")
}

// GetSettingsPath returns the path to the global settings.json
func GetSettingsPath() string {
	return filepath.Join(GetClaudeDir(), "settings.json")
}

// GetHooksDir returns the path to the hooks directory
func GetHooksDir() string {
	return filepath.Join(GetClaudeDir(), "hooks")
}
