package action

import (
	"context"
	"errors"
	"testing"

	"github.com/firebase/genkit/go/ai"
	"github.com/stretchr/testify/assert"

	"github.com/Zereker/memory/internal/domain"
)

func TestExtractionAction_Basic(t *testing.T) {
	t.Run("Name returns correct name", func(t *testing.T) {
		ctx := context.Background()
		_ = newTestHelper(ctx)

		action := NewExtractionAction()
		assert.Equal(t, "extraction", action.Name())
	})

	t.Run("WithStores returns same instance for chaining", func(t *testing.T) {
		ctx := context.Background()
		_ = newTestHelper(ctx)

		mockVector := NewMockVectorStore()
		mockGraph := NewMockGraphStore()

		action := NewExtractionAction()
		result := action.WithStores(mockVector, mockGraph)

		assert.Same(t, action, result, "should return same instance for chaining")
		assert.Equal(t, mockVector, action.vectorStore)
		assert.Equal(t, mockGraph, action.graphStore)
	})

	t.Run("Handle with empty messages produces no entities or edges", func(t *testing.T) {
		ctx := context.Background()
		_ = newTestHelper(ctx)

		mockVector := NewMockVectorStore()
		mockGraph := NewMockGraphStore()

		action := NewExtractionAction()
		action.WithStores(mockVector, mockGraph)

		addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
		addCtx.Messages = domain.Messages{}

		action.Handle(addCtx)

		assert.Empty(t, addCtx.Entities, "empty messages should produce no entities")
		assert.Empty(t, addCtx.Edges, "empty messages should produce no edges")
		assert.NoError(t, addCtx.Error())
	})
}

func TestExtractionAction_Handle(t *testing.T) {
	t.Run("WithMessages extracts entities and relations", func(t *testing.T) {
		ctx := context.Background()
		h := newTestHelper(ctx)
		h.setEmbedderVector([]float32{0.1, 0.2, 0.3})
		h.setModelJSON(map[string]any{
			"entities": []map[string]any{
				{"name": "张三", "type": "person", "description": "用户的朋友"},
				{"name": "星巴克", "type": "place", "description": "咖啡店"},
			},
			"relations": []map[string]any{
				{"subject": "张三", "predicate": "去过", "object": "星巴克", "fact": "张三去过星巴克"},
			},
		})

		mockVector := NewMockVectorStore()
		mockGraph := NewMockGraphStore()

		action := NewExtractionAction()
		action.WithStores(mockVector, mockGraph)

		addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
		addCtx.Messages = domain.Messages{
			{Role: domain.RoleUser, Content: "张三昨天去了星巴克"},
			{Role: domain.RoleAssistant, Content: "好的，我记住了"},
		}

		action.Handle(addCtx)

		assert.Len(t, addCtx.Entities, 2, "should extract 2 entities")
		assert.Len(t, addCtx.Edges, 1, "should extract 1 edge")
		assert.NoError(t, addCtx.Error())
	})

	t.Run("NoStores still extracts entities", func(t *testing.T) {
		ctx := context.Background()
		h := newTestHelper(ctx)
		h.setEmbedderVector([]float32{0.1, 0.2, 0.3})
		h.setModelJSON(map[string]any{
			"entities": []map[string]any{
				{"name": "张三", "type": "person"},
			},
			"relations": []map[string]any{},
		})

		action := NewExtractionAction()
		action.WithStores(nil, nil)

		addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
		addCtx.Messages = domain.Messages{
			{Role: domain.RoleUser, Content: "测试消息"},
		}

		action.Handle(addCtx)

		assert.Len(t, addCtx.Entities, 1, "should still extract entities without stores")
		assert.NoError(t, addCtx.Error())
	})

	t.Run("StoreEntityError skips entity without aborting", func(t *testing.T) {
		ctx := context.Background()
		h := newTestHelper(ctx)
		h.setEmbedderVector([]float32{0.1, 0.2, 0.3})
		h.setModelJSON(map[string]any{
			"entities": []map[string]any{
				{"name": "张三", "type": "person"},
			},
			"relations": []map[string]any{},
		})

		mockVector := NewMockVectorStore()
		mockGraph := NewMockGraphStore()
		mockGraph.MergeNodeFunc = func(ctx context.Context, labels []string, mergeKey string, mergeValue any, properties map[string]any) error {
			return assert.AnError
		}

		action := NewExtractionAction()
		action.WithStores(mockVector, mockGraph)

		addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
		addCtx.Messages = domain.Messages{{Role: domain.RoleUser, Content: "测试"}}

		action.Handle(addCtx)

		assert.Empty(t, addCtx.Entities, "entity store error should skip entity")
		assert.NoError(t, addCtx.Error(), "should not abort chain")
	})

	t.Run("WithEpisodes links edges to episodes", func(t *testing.T) {
		ctx := context.Background()
		h := newTestHelper(ctx)
		h.setEmbedderVector([]float32{0.1, 0.2, 0.3})
		h.setModelJSON(map[string]any{
			"entities": []map[string]any{
				{"name": "张三", "type": "person"},
				{"name": "北京", "type": "place"},
			},
			"relations": []map[string]any{
				{"subject": "张三", "predicate": "住在", "object": "北京", "fact": "张三住在北京"},
			},
		})

		mockVector := NewMockVectorStore()
		mockGraph := NewMockGraphStore()

		action := NewExtractionAction()
		action.WithStores(mockVector, mockGraph)

		addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
		addCtx.Messages = domain.Messages{
			{Role: domain.RoleUser, Content: "测试消息"},
		}
		addCtx.AddEpisodes(domain.Episode{ID: "ep_1"}, domain.Episode{ID: "ep_2"})

		action.Handle(addCtx)

		assert.Len(t, addCtx.Edges, 1)
		assert.Contains(t, addCtx.Edges[0].EpisodeIDs, "ep_1")
		assert.Contains(t, addCtx.Edges[0].EpisodeIDs, "ep_2")
	})
}

func TestExtractionResult_Types(t *testing.T) {
	result := ExtractionResult{
		Entities: []ExtractedEntity{
			{Name: "test", Type: "person", Description: "desc"},
		},
		Relations: []ExtractedRelation{
			{Subject: "a", Predicate: "rel", Object: "b", Fact: "fact"},
		},
	}

	assert.Len(t, result.Entities, 1)
	assert.Len(t, result.Relations, 1)
	assert.Equal(t, "test", result.Entities[0].Name)
	assert.Equal(t, "a", result.Relations[0].Subject)
}

func TestExtractionAction_Handle_Errors(t *testing.T) {
	t.Run("LLMExtractionError aborts chain", func(t *testing.T) {
		ctx := context.Background()
		h := newTestHelper(ctx)
		h.MockPlugin.SetModelResponse("doubao-pro-32k", func(ctx context.Context, req *ai.ModelRequest) (*ai.ModelResponse, error) {
			return nil, errors.New("extraction LLM failed")
		})

		mockVector := NewMockVectorStore()
		mockGraph := NewMockGraphStore()

		action := NewExtractionAction()
		action.WithStores(mockVector, mockGraph)

		addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
		addCtx.Messages = domain.Messages{
			{Role: domain.RoleUser, Content: "测试消息"},
		}

		action.Handle(addCtx)

		assert.Error(t, addCtx.Error())
		assert.Contains(t, addCtx.Error().Error(), "extraction failed")
	})

	t.Run("EntityEmbeddingFailure creates entity without embedding", func(t *testing.T) {
		ctx := context.Background()
		h := newTestHelper(ctx)
		h.setModelJSON(map[string]any{
			"entities": []map[string]any{
				{"name": "张三", "type": "person"},
			},
			"relations": []map[string]any{},
		})
		h.MockPlugin.SetEmbedderResponse("doubao-embedding-text-240715", func(ctx context.Context, req *ai.EmbedRequest) (*ai.EmbedResponse, error) {
			return nil, errors.New("embedding failed")
		})

		mockVector := NewMockVectorStore()
		mockGraph := NewMockGraphStore()

		action := NewExtractionAction()
		action.WithStores(mockVector, mockGraph)

		addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
		addCtx.Messages = domain.Messages{
			{Role: domain.RoleUser, Content: "测试消息"},
		}

		action.Handle(addCtx)

		assert.Len(t, addCtx.Entities, 1)
		assert.Empty(t, addCtx.Entities[0].Embedding, "entity should have no embedding")
		assert.NoError(t, addCtx.Error())
	})

	t.Run("EdgeEmbeddingFailure creates edge without embedding", func(t *testing.T) {
		ctx := context.Background()
		h := newTestHelper(ctx)
		h.setModelJSON(map[string]any{
			"entities": []map[string]any{
				{"name": "张三", "type": "person"},
				{"name": "北京", "type": "place"},
			},
			"relations": []map[string]any{
				{"subject": "张三", "predicate": "住在", "object": "北京", "fact": "张三住在北京"},
			},
		})

		callCount := 0
		h.MockPlugin.SetEmbedderResponse("doubao-embedding-text-240715", func(ctx context.Context, req *ai.EmbedRequest) (*ai.EmbedResponse, error) {
			callCount++
			if callCount <= 2 {
				return &ai.EmbedResponse{
					Embeddings: []*ai.Embedding{
						{Embedding: []float32{0.1, 0.2, 0.3}},
					},
				}, nil
			}
			return nil, errors.New("edge embedding failed")
		})

		mockVector := NewMockVectorStore()
		mockGraph := NewMockGraphStore()

		action := NewExtractionAction()
		action.WithStores(mockVector, mockGraph)

		addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
		addCtx.Messages = domain.Messages{
			{Role: domain.RoleUser, Content: "张三住在北京"},
		}

		action.Handle(addCtx)

		assert.Len(t, addCtx.Entities, 2)
		assert.Len(t, addCtx.Edges, 1)
		assert.Empty(t, addCtx.Edges[0].Embedding, "edge should have no embedding")
	})

	t.Run("EdgeReferencesUnknownEntity skips edge", func(t *testing.T) {
		ctx := context.Background()
		h := newTestHelper(ctx)
		h.setEmbedderVector([]float32{0.1, 0.2, 0.3})
		h.setModelJSON(map[string]any{
			"entities": []map[string]any{
				{"name": "张三", "type": "person"},
			},
			"relations": []map[string]any{
				{"subject": "张三", "predicate": "认识", "object": "李四", "fact": "张三认识李四"},
			},
		})

		mockVector := NewMockVectorStore()
		mockGraph := NewMockGraphStore()

		action := NewExtractionAction()
		action.WithStores(mockVector, mockGraph)

		addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
		addCtx.Messages = domain.Messages{
			{Role: domain.RoleUser, Content: "测试消息"},
		}

		action.Handle(addCtx)

		assert.Len(t, addCtx.Entities, 1)
		assert.Empty(t, addCtx.Edges, "edge with unknown entity should be skipped")
	})

	t.Run("CreateRelationshipError skips edge", func(t *testing.T) {
		ctx := context.Background()
		h := newTestHelper(ctx)
		h.setEmbedderVector([]float32{0.1, 0.2, 0.3})
		h.setModelJSON(map[string]any{
			"entities": []map[string]any{
				{"name": "张三", "type": "person"},
				{"name": "北京", "type": "place"},
			},
			"relations": []map[string]any{
				{"subject": "张三", "predicate": "住在", "object": "北京", "fact": "张三住在北京"},
			},
		})

		mockVector := NewMockVectorStore()
		mockGraph := NewMockGraphStore()
		mockGraph.CreateRelationshipFunc = func(ctx context.Context, fromLabel, fromKey string, fromValue any, toLabel, toKey string, toValue any, relType string, properties map[string]any) error {
			return errors.New("relationship creation failed")
		}

		action := NewExtractionAction()
		action.WithStores(mockVector, mockGraph)

		addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
		addCtx.Messages = domain.Messages{
			{Role: domain.RoleUser, Content: "测试消息"},
		}

		action.Handle(addCtx)

		assert.Len(t, addCtx.Entities, 2)
		assert.Empty(t, addCtx.Edges, "edge should be skipped on relationship error")
	})

	t.Run("EdgeReferencesUnstoredEntity skips edge", func(t *testing.T) {
		ctx := context.Background()
		h := newTestHelper(ctx)
		h.setEmbedderVector([]float32{0.1, 0.2, 0.3})
		h.setModelJSON(map[string]any{
			"entities": []map[string]any{
				{"name": "张三", "type": "person"},
				{"name": "北京", "type": "place"},
			},
			"relations": []map[string]any{
				{"subject": "张三", "predicate": "住在", "object": "北京", "fact": "张三住在北京"},
			},
		})

		mockVector := NewMockVectorStore()
		mockGraph := NewMockGraphStore()
		mergeCallCount := 0
		mockGraph.MergeNodeFunc = func(ctx context.Context, labels []string, mergeKey string, mergeValue any, properties map[string]any) error {
			mergeCallCount++
			if mergeCallCount == 1 {
				return errors.New("first entity store failed")
			}
			return nil
		}

		action := NewExtractionAction()
		action.WithStores(mockVector, mockGraph)

		addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
		addCtx.Messages = domain.Messages{
			{Role: domain.RoleUser, Content: "测试消息"},
		}

		action.Handle(addCtx)

		assert.Len(t, addCtx.Entities, 1, "only second entity should be stored")
		assert.Empty(t, addCtx.Edges, "edge should be skipped when source entity failed")
	})
}
