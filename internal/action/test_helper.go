package action

import (
	"context"

	pkggenkit "github.com/Zereker/memory/pkg/genkit"
)

// TestHelper provides utilities for testing actions with MockPlugin
type TestHelper struct {
	MockPlugin *pkggenkit.MockPlugin
}

// NewTestHelper creates a new test helper with MockPlugin initialized
// Must be called BEFORE creating any actions
func NewTestHelper(ctx context.Context) *TestHelper {
	mockPlugin := pkggenkit.InitForTest(ctx, pkggenkit.MockConfig{
		Provider: "ark",
		Models: []pkggenkit.ModelConfig{
			{Name: "doubao-pro-32k", Type: pkggenkit.ModelTypeLLM, Model: "doubao-pro-32k"},
			{Name: "doubao-embedding-text-240715", Type: pkggenkit.ModelTypeEmbedding, Model: "doubao-embedding", Dim: 4096},
		},
	}, "prompts")

	return &TestHelper{
		MockPlugin: mockPlugin,
	}
}

// SetEmbedderVector sets the vector response for the default embedder
func (h *TestHelper) SetEmbedderVector(vector []float32) {
	h.MockPlugin.SetEmbedderVectorResponse("doubao-embedding-text-240715", vector)
}

// SetModelJSON sets the JSON response for the default model
func (h *TestHelper) SetModelJSON(response any) {
	h.MockPlugin.SetModelJSONResponse("doubao-pro-32k", response)
}

// NewSummaryMemoryAction creates a SummaryMemoryAction with the mock genkit
func (h *TestHelper) NewSummaryMemoryAction() *SummaryMemoryAction {
	return NewSummaryMemoryAction()
}

// NewEventExtractionAction creates an EventExtractionAction with the mock genkit
func (h *TestHelper) NewEventExtractionAction() *EventExtractionAction {
	return NewEventExtractionAction()
}

// NewCognitiveRetrievalAction creates a CognitiveRetrievalAction with the mock genkit
func (h *TestHelper) NewCognitiveRetrievalAction() *CognitiveRetrievalAction {
	return NewCognitiveRetrievalAction()
}
