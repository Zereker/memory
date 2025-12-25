package action

import (
	"context"
	"testing"

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
