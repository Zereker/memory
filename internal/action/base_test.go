package action

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/Zereker/memory/internal/domain"
	pkggenkit "github.com/Zereker/memory/pkg/genkit"
	"github.com/Zereker/memory/pkg/vector"
)

// ============================================================================
// Test Helper
// ============================================================================

// testHelper provides utilities for testing actions with MockPlugin
type testHelper struct {
	MockPlugin *pkggenkit.MockPlugin
}

// newTestHelper creates a new test helper with MockPlugin initialized
func newTestHelper(ctx context.Context) *testHelper {
	mockPlugin := pkggenkit.InitForTest(ctx, pkggenkit.MockConfig{
		Provider: "ark",
		Models: []pkggenkit.ModelConfig{
			{Name: "doubao-pro-32k", Type: pkggenkit.ModelTypeLLM, Model: "doubao-pro-32k"},
			{Name: "doubao-embedding-text-240715", Type: pkggenkit.ModelTypeEmbedding, Model: "doubao-embedding", Dim: 4096},
		},
	}, "prompts")

	return &testHelper{MockPlugin: mockPlugin}
}

func (h *testHelper) setEmbedderVector(vector []float32) {
	h.MockPlugin.SetEmbedderVectorResponse("doubao-embedding-text-240715", vector)
}

func (h *testHelper) setModelJSON(response any) {
	h.MockPlugin.SetModelJSONResponse("doubao-pro-32k", response)
}

// ============================================================================
// Mock Stores
// ============================================================================

// MockVectorStore 用于测试的向量存储 mock
type MockVectorStore struct {
	StoreFunc  func(ctx context.Context, id string, doc map[string]any) error
	SearchFunc func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error)

	StoreCalls  []struct{ ID string; Doc map[string]any }
	SearchCalls []vector.SearchQuery
}

func NewMockVectorStore() *MockVectorStore {
	return &MockVectorStore{
		StoreFunc: func(ctx context.Context, id string, doc map[string]any) error {
			return nil
		},
		SearchFunc: func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
			return nil, nil
		},
	}
}

func (m *MockVectorStore) Store(ctx context.Context, id string, doc map[string]any) error {
	m.StoreCalls = append(m.StoreCalls, struct{ ID string; Doc map[string]any }{id, doc})
	return m.StoreFunc(ctx, id, doc)
}

func (m *MockVectorStore) Search(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
	m.SearchCalls = append(m.SearchCalls, query)
	return m.SearchFunc(ctx, query)
}

// MockGraphStore 用于测试的图存储 mock
type MockGraphStore struct {
	MergeNodeFunc          func(ctx context.Context, labels []string, mergeKey string, mergeValue any, properties map[string]any) error
	CreateRelationshipFunc func(ctx context.Context, fromLabel, fromKey string, fromValue any, toLabel, toKey string, toValue any, relType string, properties map[string]any) error
	TraverseFunc           func(ctx context.Context, startLabel, startKey string, startValue any, relTypes []string, direction string, maxDepth, limit int) ([]map[string]any, error)

	MergeNodeCalls          []map[string]any
	CreateRelationshipCalls []map[string]any
	TraverseCalls           []map[string]any
}

func NewMockGraphStore() *MockGraphStore {
	return &MockGraphStore{
		MergeNodeFunc: func(ctx context.Context, labels []string, mergeKey string, mergeValue any, properties map[string]any) error {
			return nil
		},
		CreateRelationshipFunc: func(ctx context.Context, fromLabel, fromKey string, fromValue any, toLabel, toKey string, toValue any, relType string, properties map[string]any) error {
			return nil
		},
		TraverseFunc: func(ctx context.Context, startLabel, startKey string, startValue any, relTypes []string, direction string, maxDepth, limit int) ([]map[string]any, error) {
			return nil, nil
		},
	}
}

func (m *MockGraphStore) MergeNode(ctx context.Context, labels []string, mergeKey string, mergeValue any, properties map[string]any) error {
	m.MergeNodeCalls = append(m.MergeNodeCalls, map[string]any{
		"labels":     labels,
		"mergeKey":   mergeKey,
		"mergeValue": mergeValue,
		"properties": properties,
	})
	return m.MergeNodeFunc(ctx, labels, mergeKey, mergeValue, properties)
}

func (m *MockGraphStore) CreateRelationship(ctx context.Context, fromLabel, fromKey string, fromValue any, toLabel, toKey string, toValue any, relType string, properties map[string]any) error {
	m.CreateRelationshipCalls = append(m.CreateRelationshipCalls, map[string]any{
		"fromLabel":  fromLabel,
		"fromKey":    fromKey,
		"fromValue":  fromValue,
		"toLabel":    toLabel,
		"toKey":      toKey,
		"toValue":    toValue,
		"relType":    relType,
		"properties": properties,
	})
	return m.CreateRelationshipFunc(ctx, fromLabel, fromKey, fromValue, toLabel, toKey, toValue, relType, properties)
}

func (m *MockGraphStore) Traverse(ctx context.Context, startLabel, startKey string, startValue any, relTypes []string, direction string, maxDepth, limit int) ([]map[string]any, error) {
	m.TraverseCalls = append(m.TraverseCalls, map[string]any{
		"startLabel": startLabel,
		"startKey":   startKey,
		"startValue": startValue,
		"relTypes":   relTypes,
		"direction":  direction,
		"maxDepth":   maxDepth,
		"limit":      limit,
	})
	return m.TraverseFunc(ctx, startLabel, startKey, startValue, relTypes, direction, maxDepth, limit)
}

// ============================================================================
// BaseAction Tests
// ============================================================================

func TestNewBaseAction(t *testing.T) {
	action := NewBaseAction("test")

	assert.NotNil(t, action)
	assert.Equal(t, "test", action.name)
	assert.NotNil(t, action.logger)
}

func TestBaseAction_GenEmbedding_WithMockPlugin(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	h.setEmbedderVector([]float32{0.1, 0.2, 0.3})

	action := NewBaseAction("test")
	embedding, err := action.GenEmbedding(ctx, "ark/doubao-embedding-text-240715", "hello")

	assert.NoError(t, err)
	assert.Equal(t, []float32{0.1, 0.2, 0.3}, embedding)
}

func TestBaseAction_Generate_WithMockPlugin(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	h.setModelJSON(map[string]any{
		"topic": "测试主题",
	})

	action := NewBaseAction("test")
	addCtx := domain.NewAddContext(ctx, "agent", "user", "session")

	var result TopicResult
	err := action.Generate(addCtx, "topic", map[string]any{
		"content":  "测试内容",
		"language": "中文",
	}, &result)

	assert.NoError(t, err)
	assert.Equal(t, "测试主题", result.Topic)
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

func TestBaseAction_DocToEpisode_EmptyDoc(t *testing.T) {
	action := NewBaseAction("test")
	ep := action.DocToEpisode(map[string]any{})
	assert.NotNil(t, ep)
	assert.Empty(t, ep.ID)
}

func TestBaseAction_DocToSummary_EmptyDoc(t *testing.T) {
	action := NewBaseAction("test")
	s := action.DocToSummary(map[string]any{})
	assert.NotNil(t, s)
	assert.Empty(t, s.ID)
}

func TestBaseAction_DocToEdge_EmptyDoc(t *testing.T) {
	action := NewBaseAction("test")
	e := action.DocToEdge(map[string]any{})
	assert.NotNil(t, e)
	assert.Empty(t, e.ID)
}

func TestBaseAction_DocToEntity_EmptyDoc(t *testing.T) {
	action := NewBaseAction("test")
	entity := action.DocToEntity(map[string]any{})
	assert.NotNil(t, entity)
	assert.Empty(t, entity.ID)
}

func TestBaseAction_DocToEdge_WithStringSlice(t *testing.T) {
	action := NewBaseAction("test")
	doc := map[string]any{
		"id":          "edge_123",
		"episode_ids": []string{"ep_1", "ep_2"}, // []string instead of []any
	}

	edge := action.DocToEdge(doc)
	assert.Equal(t, "edge_123", edge.ID)
	assert.Len(t, edge.EpisodeIDs, 2)
}

func TestBaseAction_DocToSummary_WithStringSlice(t *testing.T) {
	action := NewBaseAction("test")
	doc := map[string]any{
		"id":          "sum_123",
		"episode_ids": []string{"ep_1", "ep_2"}, // []string instead of []any
	}

	summary := action.DocToSummary(doc)
	assert.Equal(t, "sum_123", summary.ID)
	assert.Len(t, summary.EpisodeIDs, 2)
}

func TestBaseAction_timeHook_AlreadyTime(t *testing.T) {
	action := NewBaseAction("test")
	now := time.Now()

	doc := map[string]any{
		"id":         "ep_123",
		"created_at": now,
	}

	ep := action.DocToEpisode(doc)
	assert.True(t, ep.CreatedAt.Equal(now))
}

func TestBaseAction_timeHook_InvalidFormat(t *testing.T) {
	action := NewBaseAction("test")
	doc := map[string]any{
		"id":         "ep_123",
		"created_at": "invalid-time-format",
	}

	ep := action.DocToEpisode(doc)

	assert.NotNil(t, ep, "should not panic on invalid time format")
	assert.True(t, ep.CreatedAt.IsZero(), "invalid format should result in zero time")
}

func TestBaseAction_timeHook_MultipleFormats(t *testing.T) {
	action := NewBaseAction("test")

	formats := []string{
		"2024-01-15T10:30:00Z",          // RFC3339
		"2024-01-15T10:30:00.123456789Z", // RFC3339Nano
		"2024-01-15T10:30:00",           // Without timezone
		"2024-01-15 10:30:00",           // Space separator
		"2024-01-15",                    // Date only
	}

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			doc := map[string]any{
				"id":         "ep_123",
				"created_at": format,
			}
			ep := action.DocToEpisode(doc)
			assert.False(t, ep.CreatedAt.IsZero(), "failed to parse: %s", format)
		})
	}
}

func TestBaseAction_float32SliceHook_NonSliceData(t *testing.T) {
	action := NewBaseAction("test")
	doc := map[string]any{
		"id":                "ep_123",
		"content_embedding": "not a slice",
	}

	ep := action.DocToEpisode(doc)
	assert.Nil(t, ep.Embedding)
}

func TestBaseAction_stringSliceHook_NonSliceData(t *testing.T) {
	action := NewBaseAction("test")
	doc := map[string]any{
		"id":          "edge_123",
		"episode_ids": 123, // not a slice, not a string
	}

	edge := action.DocToEdge(doc)
	// mapstructure with WeaklyTypedInput may convert to empty slice
	assert.NotNil(t, edge)
}

func TestBaseAction_stringSliceHook_MixedTypes(t *testing.T) {
	action := NewBaseAction("test")
	doc := map[string]any{
		"id":          "edge_123",
		"episode_ids": []any{"ep_1", 123, "ep_2"}, // mixed types
	}

	edge := action.DocToEdge(doc)
	// Only strings should be extracted
	assert.Len(t, edge.EpisodeIDs, 2)
	assert.Equal(t, "ep_1", edge.EpisodeIDs[0])
	assert.Equal(t, "ep_2", edge.EpisodeIDs[1])
}

func TestBaseAction_DocToEpisode_InvalidData(t *testing.T) {
	action := NewBaseAction("test")

	doc := map[string]any{
		"id":                123,                   // should be string
		"created_at":        "invalid-date-format",
		"content_embedding": "not-a-slice",        // should be []float32
	}

	ep := action.DocToEpisode(doc)

	// Note: DocToEpisode returns empty struct on parse failure
	// Caller cannot distinguish between empty data and parse error
	assert.NotNil(t, ep, "should return empty struct, not nil")
	assert.Empty(t, ep.ID, "invalid type should result in empty ID")
	assert.Nil(t, ep.Embedding, "invalid type should result in nil embedding")
}

// ============================================================================
// Additional Coverage Tests for base.go
// ============================================================================

func TestBaseAction_GenEmbedding_EmptyResponse(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	// Set empty embedding response
	h.MockPlugin.SetEmbedderVectorResponse("doubao-embedding-text-240715", []float32{})

	action := NewBaseAction("test")
	_, err := action.GenEmbedding(ctx, "ark/doubao-embedding-text-240715", "hello")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty embedding response")
}

func TestBaseAction_Generate_PromptNotFound(t *testing.T) {
	ctx := context.Background()
	_ = newTestHelper(ctx)

	action := NewBaseAction("test")
	addCtx := domain.NewAddContext(ctx, "agent", "user", "session")

	var result TopicResult
	err := action.Generate(addCtx, "nonexistent_prompt", map[string]any{}, &result)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "prompt not found")
}

func TestBaseAction_timeHook_NonStringNonTime(t *testing.T) {
	action := NewBaseAction("test")
	doc := map[string]any{
		"id":         "ep_123",
		"created_at": 12345, // integer, not string or time
	}

	ep := action.DocToEpisode(doc)
	// Should handle gracefully, not panic
	assert.NotNil(t, ep)
	assert.True(t, ep.CreatedAt.IsZero())
}

func TestBaseAction_float32SliceHook_NonFloat64InSlice(t *testing.T) {
	action := NewBaseAction("test")
	doc := map[string]any{
		"id":                "ep_123",
		"content_embedding": []any{"string", 0.2, 0.3}, // first element is not float64
	}

	ep := action.DocToEpisode(doc)
	assert.NotNil(t, ep)
	// The first element should be 0 (default for failed conversion)
	if len(ep.Embedding) > 0 {
		assert.Equal(t, float32(0), ep.Embedding[0])
	}
}

func TestBaseAction_DocToEntity_WithoutEntityType(t *testing.T) {
	action := NewBaseAction("test")
	doc := map[string]any{
		"id":   "ent_123",
		"name": "张三",
		// No entity_type field
	}

	entity := action.DocToEntity(doc)
	assert.Equal(t, "ent_123", entity.ID)
	assert.Equal(t, "张三", entity.Name)
	assert.Empty(t, entity.Type)
}

// TestBaseAction_DocConversion_TimeFields tests time field handling across all Doc converters
func TestBaseAction_DocConversion_TimeFields(t *testing.T) {
	action := NewBaseAction("test")
	now := time.Now()

	t.Run("Summary", func(t *testing.T) {
		doc := map[string]any{"id": "sum_123", "created_at": now, "updated_at": now}
		summary := action.DocToSummary(doc)
		assert.True(t, summary.CreatedAt.Equal(now))
		assert.True(t, summary.UpdatedAt.Equal(now))
	})

	t.Run("Edge", func(t *testing.T) {
		doc := map[string]any{"id": "edge_123", "created_at": now}
		edge := action.DocToEdge(doc)
		assert.True(t, edge.CreatedAt.Equal(now))
	})

	t.Run("Entity", func(t *testing.T) {
		doc := map[string]any{"id": "ent_123", "created_at": now, "updated_at": now}
		entity := action.DocToEntity(doc)
		assert.True(t, entity.CreatedAt.Equal(now))
		assert.True(t, entity.UpdatedAt.Equal(now))
	})
}
