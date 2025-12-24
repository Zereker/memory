package server

import (
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"

	"github.com/Zereker/memory/pkg/genkit"
	"github.com/Zereker/memory/pkg/log"
	"github.com/Zereker/memory/pkg/relation"
	"github.com/Zereker/memory/pkg/vector"
)

// Config holds all configuration values
type Config struct {
	Server  ServerConfig          `toml:"server"`
	Log     log.Config            `toml:"log"`
	Models  genkit.Config         `toml:"genkit"`
	Storage  vector.OpenSearchConfig  `toml:"storage"`
	Postgres relation.PostgresConfig `toml:"postgres"`
}

// ServerConfig contains server configuration
type ServerConfig struct {
	Mode string `toml:"mode"` // http, mcp, or both
	Port int    `toml:"port"`
}

// AgentConfig defines agent configuration
type AgentConfig struct {
	Name        string   `toml:"name" json:"name"`
	Description string   `toml:"description" json:"description"`
	Enabled     bool     `toml:"enabled" json:"enabled"`
	Actions     []string `toml:"actions" json:"actions"`
}

// Validate checks server configuration
func (s *ServerConfig) Validate() error {
	if s.Mode == "" {
		s.Mode = "http" // default mode
	}
	switch s.Mode {
	case "http", "mcp", "both":
		// valid
	default:
		return fmt.Errorf("invalid mode: %s, must be http, mcp, or both", s.Mode)
	}
	if s.Port <= 0 || s.Port > 65535 {
		return fmt.Errorf("port is required and must be between 1 and 65535")
	}
	return nil
}

// Validate checks agent configuration
func (c *AgentConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("name is required")
	}
	if len(c.Actions) == 0 {
		return fmt.Errorf("at least one action is required")
	}
	return nil
}

// Validate checks all configuration fields
func (c *Config) Validate() error {
	if err := c.Server.Validate(); err != nil {
		return fmt.Errorf("server: %w", err)
	}

	if err := c.Log.Validate(); err != nil {
		return fmt.Errorf("log: %w", err)
	}

	if err := c.Models.Validate(); err != nil {
		return fmt.Errorf("genkit: %w", err)
	}

	if err := c.Storage.Validate(); err != nil {
		return fmt.Errorf("storage: %w", err)
	}

	if err := c.Postgres.Validate(); err != nil {
		return fmt.Errorf("postgres: %w", err)
	}

	return nil
}

// LoadConfig reads and parses the configuration file
func LoadConfig(filename string) (Config, error) {
	var cfg Config

	data, err := os.ReadFile(filename)
	if err != nil {
		return cfg, fmt.Errorf("read config file: %w", err)
	}

	if err := toml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config file: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return cfg, fmt.Errorf("validate config: %w", err)
	}

	return cfg, nil
}
