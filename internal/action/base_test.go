package action

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/Zereker/memory/internal/domain"
)

func TestNewBaseAction(t *testing.T) {
	action := NewBaseAction("test")

	assert.NotNil(t, action)
	assert.Equal(t, "test", action.name)
	assert.NotNil(t, action.logger)
}

func TestBaseAction_WithLLMClient(t *testing.T) {
	action := NewBaseAction("test")
	mockClient := NewMockLLMClient()

	result := action.WithLLMClient(mockClient)

	assert.Same(t, action, result)
	assert.Equal(t, mockClient, action.llmClient)
}

func TestBaseAction_GenEmbedding_WithMock(t *testing.T) {
	mockClient := NewMockLLMClient()
	mockClient.GenEmbeddingFunc = func(ctx context.Context, embedderName, text string) ([]float32, error) {
		return []float32{0.1, 0.2, 0.3}, nil
	}

	action := NewBaseAction("test").WithLLMClient(mockClient)

	embedding, err := action.GenEmbedding(context.Background(), "test-embedder", "hello")

	assert.NoError(t, err)
	assert.Equal(t, []float32{0.1, 0.2, 0.3}, embedding)
	assert.Len(t, mockClient.GenEmbeddingCalls, 1)
	assert.Equal(t, "test-embedder", mockClient.GenEmbeddingCalls[0].EmbedderName)
	assert.Equal(t, "hello", mockClient.GenEmbeddingCalls[0].Text)
}

func TestBaseAction_Generate_WithMock(t *testing.T) {
	mockClient := NewMockLLMClient()
	mockClient.GenerateFunc = func(c *domain.AddContext, promptName string, input map[string]any, output any) error {
		// 模拟返回结果
		if result, ok := output.(*ExtractionResult); ok {
			result.Entities = []ExtractedEntity{{Name: "test", Type: "person"}}
		}
		return nil
	}

	action := NewBaseAction("test").WithLLMClient(mockClient)
	ctx := domain.NewAddContext(context.Background(), "agent", "user", "session")

	var result ExtractionResult
	err := action.Generate(ctx, "extraction", map[string]any{"input": "test"}, &result)

	assert.NoError(t, err)
	assert.Len(t, result.Entities, 1)
	assert.Equal(t, "test", result.Entities[0].Name)
	assert.Len(t, mockClient.GenerateCalls, 1)
	assert.Equal(t, "extraction", mockClient.GenerateCalls[0].PromptName)
}

func TestBaseAction_CosineSimilarity(t *testing.T) {
	action := NewBaseAction("test")

	tests := []struct {
		name     string
		vec1     []float32
		vec2     []float32
		expected float64
	}{
		{
			name:     "identical vectors",
			vec1:     []float32{1, 0, 0},
			vec2:     []float32{1, 0, 0},
			expected: 1.0,
		},
		{
			name:     "orthogonal vectors",
			vec1:     []float32{1, 0, 0},
			vec2:     []float32{0, 1, 0},
			expected: 0.0,
		},
		{
			name:     "opposite vectors",
			vec1:     []float32{1, 0, 0},
			vec2:     []float32{-1, 0, 0},
			expected: -1.0,
		},
		{
			name:     "empty vectors",
			vec1:     []float32{},
			vec2:     []float32{},
			expected: 0.0,
		},
		{
			name:     "different length vectors",
			vec1:     []float32{1, 2},
			vec2:     []float32{1, 2, 3},
			expected: 0.0,
		},
		{
			name:     "zero vectors",
			vec1:     []float32{0, 0, 0},
			vec2:     []float32{0, 0, 0},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := action.CosineSimilarity(tt.vec1, tt.vec2)
			assert.InDelta(t, tt.expected, result, 0.0001)
		})
	}
}

func TestBaseAction_DocToEpisode(t *testing.T) {
	action := NewBaseAction("test")

	tests := []struct {
		name     string
		doc      map[string]any
		validate func(*testing.T, *domain.Episode)
	}{
		{
			name: "basic fields",
			doc: map[string]any{
				"id":      "ep_123",
				"role":    "user",
				"name":    "张三",
				"content": "测试内容",
				"topic":   "测试主题",
			},
			validate: func(t *testing.T, ep *domain.Episode) {
				assert.Equal(t, "ep_123", ep.ID)
				assert.Equal(t, "user", ep.Role)
				assert.Equal(t, "张三", ep.Name)
				assert.Equal(t, "测试内容", ep.Content)
				assert.Equal(t, "测试主题", ep.Topic)
			},
		},
		{
			name: "time fields - RFC3339",
			doc: map[string]any{
				"id":         "ep_time",
				"created_at": "2024-01-15T10:30:00Z",
				"timestamp":  "2024-01-15T10:30:00Z",
			},
			validate: func(t *testing.T, ep *domain.Episode) {
				expected := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
				assert.True(t, ep.CreatedAt.Equal(expected))
				assert.True(t, ep.Timestamp.Equal(expected))
			},
		},
		{
			name: "embedding fields - []any",
			doc: map[string]any{
				"id":                "ep_emb",
				"content_embedding": []any{0.1, 0.2, 0.3},
				"topic_embedding":   []any{0.4, 0.5, 0.6},
			},
			validate: func(t *testing.T, ep *domain.Episode) {
				assert.Len(t, ep.Embedding, 3)
				assert.Len(t, ep.TopicEmbedding, 3)
			},
		},
		{
			name: "embedding fields - []float32",
			doc: map[string]any{
				"id":                "ep_emb_f32",
				"content_embedding": []float32{0.1, 0.2, 0.3},
			},
			validate: func(t *testing.T, ep *domain.Episode) {
				assert.Len(t, ep.Embedding, 3)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ep := action.DocToEpisode(tt.doc)
			tt.validate(t, ep)
		})
	}
}

func TestBaseAction_DocToEntity(t *testing.T) {
	action := NewBaseAction("test")

	doc := map[string]any{
		"id":          "ent_123",
		"name":        "张三",
		"entity_type": "person",
		"description": "一个人",
	}

	entity := action.DocToEntity(doc)

	assert.Equal(t, "ent_123", entity.ID)
	assert.Equal(t, "张三", entity.Name)
	assert.Equal(t, domain.EntityType("person"), entity.Type)
	assert.Equal(t, "一个人", entity.Description)
}

func TestBaseAction_DocToEdge(t *testing.T) {
	action := NewBaseAction("test")

	doc := map[string]any{
		"id":          "edge_123",
		"source_id":   "ent_1",
		"target_id":   "ent_2",
		"relation":    "knows",
		"fact":        "张三认识李四",
		"episode_ids": []any{"ep_1", "ep_2"},
	}

	edge := action.DocToEdge(doc)

	assert.Equal(t, "edge_123", edge.ID)
	assert.Equal(t, "ent_1", edge.SourceID)
	assert.Equal(t, "ent_2", edge.TargetID)
	assert.Equal(t, "knows", edge.Relation)
	assert.Equal(t, "张三认识李四", edge.Fact)
	assert.Len(t, edge.EpisodeIDs, 2)
}

func TestBaseAction_DocToSummary(t *testing.T) {
	action := NewBaseAction("test")

	doc := map[string]any{
		"id":          "sum_123",
		"agent_id":    "agent_1",
		"user_id":     "user_1",
		"content":     "摘要内容",
		"topic":       "主题",
		"episode_ids": []any{"ep_1", "ep_2"},
	}

	summary := action.DocToSummary(doc)

	assert.Equal(t, "sum_123", summary.ID)
	assert.Equal(t, "agent_1", summary.AgentID)
	assert.Equal(t, "user_1", summary.UserID)
	assert.Equal(t, "摘要内容", summary.Content)
	assert.Equal(t, "主题", summary.Topic)
	assert.Len(t, summary.EpisodeIDs, 2)
}
