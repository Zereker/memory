package genkit

import (
	"context"
	"fmt"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core/api"
	"github.com/firebase/genkit/go/plugins/compat_oai"
	"github.com/openai/openai-go/option"
)

// ArkConfig holds configuration for Ark vendor
type ArkConfig struct {
	APIKey  string        `toml:"api_key"`
	BaseURL string        `toml:"base_url"`
	Models  []ModelConfig `toml:"models"`
}

// Validate checks Ark configuration
func (c *ArkConfig) Validate() error {
	if c.APIKey == "" {
		return fmt.Errorf("api_key is required")
	}
	if c.BaseURL == "" {
		return fmt.Errorf("base_url is required")
	}
	if len(c.Models) == 0 {
		return fmt.Errorf("at least one model is required")
	}
	for i := range c.Models {
		if err := c.Models[i].Validate(i); err != nil {
			return err
		}
	}
	return nil
}

// ArkPlugin implements Genkit plugin for Volcengine Ark (Doubao)
type ArkPlugin struct {
	compat_oai.OpenAICompatible
	models []ModelConfig
}

// NewArkPlugin creates a new Ark plugin for Genkit
func NewArkPlugin(cfg ArkConfig) *ArkPlugin {
	return &ArkPlugin{
		OpenAICompatible: compat_oai.OpenAICompatible{
			APIKey:   cfg.APIKey,
			BaseURL:  cfg.BaseURL,
			Provider: "ark",
			Opts: []option.RequestOption{
				option.WithHeader("Content-Type", "application/json"),
			},
		},
		models: cfg.Models,
	}
}

// Name returns the plugin name
func (p *ArkPlugin) Name() string {
	return "ark"
}

// Init implements api.Plugin interface - registers all Ark models
func (p *ArkPlugin) Init(ctx context.Context) []api.Action {
	// Initialize the base OpenAI compatible client first
	p.OpenAICompatible.Init(ctx)

	actions := make([]api.Action, 0, len(p.models))

	for _, m := range p.models {
		switch m.Type {
		case ModelTypeLLM:
			model := p.DefineModel(p.Provider, m.Model, ai.ModelOptions{
				Label: fmt.Sprintf("Ark %s", m.Name),
				Supports: &ai.ModelSupports{
					Multiturn:  true,
					Tools:      true,
					SystemRole: true,
					Media:      false,
				},
			})
			actions = append(actions, model.(api.Action))

		case ModelTypeEmbedding:
			embedder := p.DefineEmbedder(p.Provider, m.Model, &ai.EmbedderOptions{
				Label:      fmt.Sprintf("Ark %s", m.Name),
				Dimensions: m.Dim,
			})
			actions = append(actions, embedder.(api.Action))
		}
	}

	return actions
}

