package action

import (
	"context"
	"errors"
	"testing"

	"github.com/firebase/genkit/go/ai"
	"github.com/stretchr/testify/assert"

	"github.com/Zereker/memory/internal/domain"
)

func TestExtractionAction_Name(t *testing.T) {
	ctx := context.Background()
	_ = newTestHelper(ctx) // Initialize genkit

	action := NewExtractionAction()
	assert.Equal(t, "extraction", action.Name())
}

func TestExtractionAction_WithStores(t *testing.T) {
	ctx := context.Background()
	_ = newTestHelper(ctx)

	mockVector := NewMockVectorStore()
	mockGraph := NewMockGraphStore()

	action := NewExtractionAction()
	result := action.WithStores(mockVector, mockGraph)

	assert.Same(t, action, result)
	assert.Equal(t, mockVector, action.vectorStore)
	assert.Equal(t, mockGraph, action.graphStore)
}

func TestExtractionAction_Handle_EmptyMessages(t *testing.T) {
	ctx := context.Background()
	_ = newTestHelper(ctx)

	mockVector := NewMockVectorStore()
	mockGraph := NewMockGraphStore()

	action := NewExtractionAction()
	action.WithStores(mockVector, mockGraph)

	addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
	addCtx.Messages = domain.Messages{} // 空消息

	action.Handle(addCtx)

	assert.Empty(t, addCtx.Entities)
	assert.Empty(t, addCtx.Edges)
	assert.NoError(t, addCtx.Error())
}

func TestExtractionAction_Handle_WithMessages(t *testing.T) {
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

	// 验证生成了实体和关系
	assert.Len(t, addCtx.Entities, 2)
	assert.Len(t, addCtx.Edges, 1)
	assert.NoError(t, addCtx.Error())
}

func TestExtractionAction_Handle_NoStores(t *testing.T) {
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
	action.WithStores(nil, nil) // 无存储

	addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
	addCtx.Messages = domain.Messages{
		{Role: domain.RoleUser, Content: "测试消息"},
	}

	// 不应 panic
	action.Handle(addCtx)

	// 仍然应该提取实体
	assert.Len(t, addCtx.Entities, 1)
	assert.NoError(t, addCtx.Error())
}

func TestExtractionAction_Handle_StoreEntityError(t *testing.T) {
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

	// Entity 存储失败，不应添加到 context
	assert.Empty(t, addCtx.Entities)
	assert.NoError(t, addCtx.Error()) // 不应中断整个流程
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

// ============================================================================
// Additional Coverage Tests for extraction.go
// ============================================================================

func TestExtractionAction_Handle_LLMExtractionError(t *testing.T) {
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

	// LLM error should be set
	assert.Error(t, addCtx.Error())
	assert.Contains(t, addCtx.Error().Error(), "extraction failed")
}

func TestExtractionAction_Handle_EntityEmbeddingFailure(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	h.setModelJSON(map[string]any{
		"entities": []map[string]any{
			{"name": "张三", "type": "person"},
		},
		"relations": []map[string]any{},
	})
	// Embedding fails
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

	// Entity should still be created, just without embedding
	assert.Len(t, addCtx.Entities, 1)
	assert.Empty(t, addCtx.Entities[0].Embedding)
	assert.NoError(t, addCtx.Error())
}

func TestExtractionAction_Handle_EdgeEmbeddingFailure(t *testing.T) {
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

	// First 2 calls succeed (for entities), third fails (for edge)
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

	// Entities should be stored
	assert.Len(t, addCtx.Entities, 2)
	// Edge should still be created, just without embedding
	assert.Len(t, addCtx.Edges, 1)
	assert.Empty(t, addCtx.Edges[0].Embedding)
}

func TestExtractionAction_Handle_EdgeReferencesUnknownEntity(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	h.setEmbedderVector([]float32{0.1, 0.2, 0.3})
	h.setModelJSON(map[string]any{
		"entities": []map[string]any{
			{"name": "张三", "type": "person"},
			// Missing "李四" entity
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

	// Entity stored
	assert.Len(t, addCtx.Entities, 1)
	// Edge should be skipped because "李四" is not found
	assert.Empty(t, addCtx.Edges)
}

func TestExtractionAction_Handle_StoreEntityToVectorError(t *testing.T) {
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
	mockVector.StoreFunc = func(ctx context.Context, id string, doc map[string]any) error {
		return errors.New("vector store failed")
	}
	mockGraph := NewMockGraphStore()

	action := NewExtractionAction()
	action.WithStores(mockVector, mockGraph)

	addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
	addCtx.Messages = domain.Messages{
		{Role: domain.RoleUser, Content: "测试消息"},
	}

	action.Handle(addCtx)

	// Entity should still be added to context even if vector store fails
	assert.Len(t, addCtx.Entities, 1)
	assert.NoError(t, addCtx.Error())
}

func TestExtractionAction_Handle_CreateRelationshipError(t *testing.T) {
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

	// Entities should be stored
	assert.Len(t, addCtx.Entities, 2)
	// Edge should be skipped because graph store failed
	assert.Empty(t, addCtx.Edges)
}

func TestExtractionAction_Handle_StoreEdgeToVectorError(t *testing.T) {
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

	storeCallCount := 0
	mockVector := NewMockVectorStore()
	mockVector.StoreFunc = func(ctx context.Context, id string, doc map[string]any) error {
		storeCallCount++
		// First 2 calls for entities succeed, 3rd call for edge fails
		if storeCallCount > 2 {
			return errors.New("edge vector store failed")
		}
		return nil
	}
	mockGraph := NewMockGraphStore()

	action := NewExtractionAction()
	action.WithStores(mockVector, mockGraph)

	addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
	addCtx.Messages = domain.Messages{
		{Role: domain.RoleUser, Content: "测试消息"},
	}

	action.Handle(addCtx)

	// Entities should be stored
	assert.Len(t, addCtx.Entities, 2)
	// Edge should still be added to context even if vector store fails
	assert.Len(t, addCtx.Edges, 1)
}

func TestExtractionAction_Handle_EdgeReferencesUnstoredEntity(t *testing.T) {
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
	// First entity fails to store, second succeeds
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

	// Only second entity should be stored
	assert.Len(t, addCtx.Entities, 1)
	// Edge should be skipped because source entity wasn't stored
	assert.Empty(t, addCtx.Edges)
}

func TestExtractionAction_Handle_WithEpisodes(t *testing.T) {
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
	// Add some episodes
	addCtx.AddEpisodes(domain.Episode{ID: "ep_1"}, domain.Episode{ID: "ep_2"})

	action.Handle(addCtx)

	// Edge should have episode IDs
	assert.Len(t, addCtx.Edges, 1)
	assert.Contains(t, addCtx.Edges[0].EpisodeIDs, "ep_1")
	assert.Contains(t, addCtx.Edges[0].EpisodeIDs, "ep_2")
}
