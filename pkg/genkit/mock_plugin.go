package genkit

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core/api"
)

// MockConfig holds mock plugin configuration
type MockConfig struct {
	Provider string        // Provider prefix (default: "mock"). Use "ark" to match real model names.
	Models   []ModelConfig
}

// MockPlugin implements a test-only genkit plugin with configurable responses
type MockPlugin struct {
	mu sync.RWMutex

	// provider prefix for model names
	provider string
	// modelResponses maps model name to response function
	modelResponses map[string]func(ctx context.Context, req *ai.ModelRequest) (*ai.ModelResponse, error)
	// embedderResponses maps embedder name to response function
	embedderResponses map[string]func(ctx context.Context, req *ai.EmbedRequest) (*ai.EmbedResponse, error)

	// models to register
	models []ModelConfig
}

// NewMockPlugin creates a new mock plugin for testing
func NewMockPlugin(cfg MockConfig) *MockPlugin {
	provider := cfg.Provider
	if provider == "" {
		provider = "mock"
	}
	return &MockPlugin{
		provider:          provider,
		models:            cfg.Models,
		modelResponses:    make(map[string]func(ctx context.Context, req *ai.ModelRequest) (*ai.ModelResponse, error)),
		embedderResponses: make(map[string]func(ctx context.Context, req *ai.EmbedRequest) (*ai.EmbedResponse, error)),
	}
}

// Name returns the plugin name
func (p *MockPlugin) Name() string {
	return "mock"
}

// Init implements api.Plugin interface - registers all mock models/embedders
func (p *MockPlugin) Init(ctx context.Context) []api.Action {
	actions := make([]api.Action, 0, len(p.models))

	for _, m := range p.models {
		switch m.Type {
		case ModelTypeLLM:
			model := p.defineModel(m)
			actions = append(actions, model.(api.Action))

		case ModelTypeEmbedding:
			embedder := p.defineEmbedder(m)
			actions = append(actions, embedder.(api.Action))
		}
	}

	return actions
}

// defineModel creates a mock model
func (p *MockPlugin) defineModel(m ModelConfig) ai.Model {
	name := fmt.Sprintf("%s/%s", p.provider, m.Name)
	return ai.NewModel(name, &ai.ModelOptions{
		Label: fmt.Sprintf("Mock %s", m.Name),
		Supports: &ai.ModelSupports{
			Multiturn:  true,
			Tools:      true,
			SystemRole: true,
			Media:      false,
		},
	}, func(ctx context.Context, req *ai.ModelRequest, cb ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		p.mu.RLock()
		fn, ok := p.modelResponses[m.Name]
		p.mu.RUnlock()

		if ok && fn != nil {
			return fn(ctx, req)
		}

		// Default: echo the last user message
		textResponse := ""
		for _, msg := range req.Messages {
			if msg.Role == ai.RoleUser {
				textResponse = msg.Text()
			}
		}

		return &ai.ModelResponse{
			Request: req,
			Message: ai.NewModelTextMessage(textResponse),
			Usage: &ai.GenerationUsage{
				InputTokens:  10,
				OutputTokens: 5,
			},
		}, nil
	})
}

// defineEmbedder creates a mock embedder
func (p *MockPlugin) defineEmbedder(m ModelConfig) ai.Embedder {
	name := fmt.Sprintf("%s/%s", p.provider, m.Name)
	return ai.NewEmbedder(name, &ai.EmbedderOptions{
		Label:      fmt.Sprintf("Mock %s", m.Name),
		Dimensions: m.Dim,
	}, func(ctx context.Context, req *ai.EmbedRequest) (*ai.EmbedResponse, error) {
		p.mu.RLock()
		fn, ok := p.embedderResponses[m.Name]
		p.mu.RUnlock()

		if ok && fn != nil {
			return fn(ctx, req)
		}

		// Default: return zero vectors
		embeddings := make([]*ai.Embedding, len(req.Input))
		for i := range req.Input {
			embeddings[i] = &ai.Embedding{
				Embedding: make([]float32, m.Dim),
			}
		}

		return &ai.EmbedResponse{
			Embeddings: embeddings,
		}, nil
	})
}

// SetModelResponse sets a custom response function for a model
func (p *MockPlugin) SetModelResponse(modelName string, fn func(ctx context.Context, req *ai.ModelRequest) (*ai.ModelResponse, error)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.modelResponses[modelName] = fn
}

// SetEmbedderResponse sets a custom response function for an embedder
func (p *MockPlugin) SetEmbedderResponse(embedderName string, fn func(ctx context.Context, req *ai.EmbedRequest) (*ai.EmbedResponse, error)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.embedderResponses[embedderName] = fn
}

// SetModelJSONResponse is a helper to set a model response that returns structured JSON
func (p *MockPlugin) SetModelJSONResponse(modelName string, response any) {
	p.SetModelResponse(modelName, func(ctx context.Context, req *ai.ModelRequest) (*ai.ModelResponse, error) {
		data, err := json.Marshal(response)
		if err != nil {
			return nil, err
		}
		return &ai.ModelResponse{
			Request: req,
			Message: ai.NewModelTextMessage(string(data)),
			Usage: &ai.GenerationUsage{
				InputTokens:  10,
				OutputTokens: 5,
			},
		}, nil
	})
}

// SetEmbedderVectorResponse is a helper to set an embedder response with a specific vector
func (p *MockPlugin) SetEmbedderVectorResponse(embedderName string, vector []float32) {
	p.SetEmbedderResponse(embedderName, func(ctx context.Context, req *ai.EmbedRequest) (*ai.EmbedResponse, error) {
		embeddings := make([]*ai.Embedding, len(req.Input))
		for i := range req.Input {
			embeddings[i] = &ai.Embedding{
				Embedding: vector,
			}
		}
		return &ai.EmbedResponse{
			Embeddings: embeddings,
		}, nil
	})
}

// DefaultMockConfig returns a default mock config for testing
func DefaultMockConfig() MockConfig {
	return MockConfig{
		Models: []ModelConfig{
			{Name: "test-llm", Type: ModelTypeLLM, Model: "test-llm"},
			{Name: "test-embedding", Type: ModelTypeEmbedding, Model: "test-embedding", Dim: 1536},
		},
	}
}
