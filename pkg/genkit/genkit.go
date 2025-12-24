package genkit

import (
	"context"
	"fmt"

	"github.com/firebase/genkit/go/core/api"
	"github.com/firebase/genkit/go/genkit"
	"github.com/pkg/errors"
)

type ModelType string

// Model type constants
const (
	ModelTypeLLM       ModelType = "llm"
	ModelTypeEmbedding ModelType = "embedding"
)

// ModelConfig holds configuration for a single model (shared by all vendors)
type ModelConfig struct {
	Name    string    `toml:"name"`     // Model name for registration (e.g., "doubao-lite", "doubao-pro-32k")
	Type    ModelType `toml:"type"`     // ModelTypeLLM or ModelTypeEmbedding
	Model   string    `toml:"model"`    // Actual model identifier
	BaseURL string    `toml:"base_url"` // Override base URL for this model (optional)
	Dim     int       `toml:"dim"`      // Embedding dimension (required for embedding models)
}

// Validate validates a model config
func (m *ModelConfig) Validate(index int) error {
	if m.Name == "" {
		return fmt.Errorf("models[%d].name is required", index)
	}

	if m.Type != ModelTypeLLM && m.Type != ModelTypeEmbedding {
		return fmt.Errorf("models[%d].type must be '%s' or '%s'", index, ModelTypeLLM, ModelTypeEmbedding)
	}

	if m.Model == "" {
		return fmt.Errorf("models[%d].model is required", index)
	}

	if m.Type == ModelTypeEmbedding && m.Dim <= 0 {
		return fmt.Errorf("models[%d].dim is required for embedding model", index)
	}

	return nil
}

// Config holds unified genkit configuration with all vendors
type Config struct {
	Ark       ArkConfig `toml:"ark"`
	PromptDir string    `toml:"prompt_dir"`
}

// Validate checks genkit configuration
func (c *Config) Validate() error {
	// PromptDir is optional - prompts can be defined in Go code

	if len(c.Ark.Models) > 0 {
		if err := c.Ark.Validate(); err != nil {
			return fmt.Errorf("ark: %w", err)
		}
	}

	return nil
}

var g *genkit.Genkit

// Init initializes the genkit package with multi-vendor config
func Init(ctx context.Context, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return errors.WithMessage(err, "invalid config")
	}

	var plugins []api.Plugin

	if len(cfg.Ark.Models) > 0 {
		plugins = append(plugins, NewArkPlugin(cfg.Ark))
	}

	g = genkit.Init(ctx,
		genkit.WithPlugins(plugins...),
		genkit.WithPromptDir(cfg.PromptDir),
	)

	return nil
}

// InitForTest initializes genkit with a mock plugin for testing.
// Returns the mock plugin for configuring responses.
func InitForTest(ctx context.Context, cfg MockConfig, promptDir string) *MockPlugin {
	mockPlugin := NewMockPlugin(cfg)

	g = genkit.Init(ctx,
		genkit.WithPlugins(mockPlugin),
		genkit.WithPromptDir(promptDir),
	)

	return mockPlugin
}

// InitWithPlugins initializes genkit with custom plugins (for testing or custom setups)
func InitWithPlugins(ctx context.Context, plugins []api.Plugin, promptDir string) {
	g = genkit.Init(ctx,
		genkit.WithPlugins(plugins...),
		genkit.WithPromptDir(promptDir),
	)
}

// Genkit returns the Genkit instance
func Genkit() *genkit.Genkit {
	return g
}
