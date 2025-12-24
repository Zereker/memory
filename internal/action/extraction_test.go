package action

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Zereker/memory/internal/domain"
)

func TestExtractionAction_Name(t *testing.T) {
	action := NewExtractionAction()
	assert.Equal(t, "extraction", action.Name())
}

func TestExtractionAction_WithStores(t *testing.T) {
	mockVector := NewMockVectorStore()
	mockGraph := NewMockGraphStore()

	action := NewExtractionAction()
	result := action.WithStores(mockVector, mockGraph)

	assert.Same(t, action, result)
	assert.Equal(t, mockVector, action.vectorStore)
	assert.Equal(t, mockGraph, action.graphStore)
}

func TestExtractionAction_Handle_EmptyMessages(t *testing.T) {
	mockLLM := NewMockLLMClient()
	mockVector := NewMockVectorStore()
	mockGraph := NewMockGraphStore()

	action := NewExtractionAction()
	action.WithLLMClient(mockLLM)
	action.WithStores(mockVector, mockGraph)

	ctx := domain.NewAddContext(context.Background(), "agent", "user", "session")
	ctx.Messages = domain.Messages{} // 空消息

	action.Handle(ctx)

	// 应该直接跳过，不调用 LLM
	assert.Empty(t, mockLLM.GenerateCalls)
	assert.Empty(t, ctx.Entities)
	assert.Empty(t, ctx.Edges)
	assert.NoError(t, ctx.Error())
}

func TestExtractionAction_Handle_WithMessages(t *testing.T) {
	mockLLM := NewMockLLMClient()
	mockLLM.GenerateFunc = func(c *domain.AddContext, promptName string, input map[string]any, output any) error {
		if promptName == "extraction" {
			result := ExtractionResult{
				Entities: []ExtractedEntity{
					{Name: "张三", Type: "person", Description: "用户的朋友"},
					{Name: "星巴克", Type: "place", Description: "咖啡店"},
				},
				Relations: []ExtractedRelation{
					{Subject: "张三", Predicate: "去过", Object: "星巴克", Fact: "张三去过星巴克"},
				},
			}
			data, _ := json.Marshal(result)
			return json.Unmarshal(data, output)
		}
		return nil
	}

	mockVector := NewMockVectorStore()
	mockGraph := NewMockGraphStore()

	action := NewExtractionAction()
	action.WithLLMClient(mockLLM)
	action.WithStores(mockVector, mockGraph)

	ctx := domain.NewAddContext(context.Background(), "agent", "user", "session")
	ctx.Messages = domain.Messages{
		{Role: "user", Content: "张三昨天去了星巴克"},
		{Role: "assistant", Content: "好的，我记住了"},
	}

	action.Handle(ctx)

	// 验证调用了 LLM
	assert.Len(t, mockLLM.GenerateCalls, 1)
	assert.Equal(t, "extraction", mockLLM.GenerateCalls[0].PromptName)

	// 验证生成了 embedding
	assert.True(t, len(mockLLM.GenEmbeddingCalls) > 0)

	// 验证存储了实体
	assert.Len(t, mockGraph.MergeNodeCalls, 2) // 2 个实体

	// 验证存储了关系
	assert.Len(t, mockGraph.CreateRelationshipCalls, 1)

	// 验证 context 中添加了实体和边
	assert.Len(t, ctx.Entities, 2)
	assert.Len(t, ctx.Edges, 1)
	assert.NoError(t, ctx.Error())
}

func TestExtractionAction_Handle_LLMError(t *testing.T) {
	mockLLM := NewMockLLMClient()
	mockLLM.GenerateFunc = func(c *domain.AddContext, promptName string, input map[string]any, output any) error {
		return errors.New("LLM error")
	}

	action := NewExtractionAction()
	action.WithLLMClient(mockLLM)
	action.WithStores(nil, nil)

	ctx := domain.NewAddContext(context.Background(), "agent", "user", "session")
	ctx.Messages = domain.Messages{
		{Role: "user", Content: "测试消息"},
	}

	action.Handle(ctx)

	// 应该设置错误
	assert.Error(t, ctx.Error())
	assert.True(t, ctx.IsAborted())
}

func TestExtractionAction_Handle_NoStores(t *testing.T) {
	mockLLM := NewMockLLMClient()
	mockLLM.GenerateFunc = func(c *domain.AddContext, promptName string, input map[string]any, output any) error {
		if promptName == "extraction" {
			result := ExtractionResult{
				Entities: []ExtractedEntity{
					{Name: "张三", Type: "person"},
				},
				Relations: []ExtractedRelation{},
			}
			data, _ := json.Marshal(result)
			return json.Unmarshal(data, output)
		}
		return nil
	}

	action := NewExtractionAction()
	action.WithLLMClient(mockLLM)
	action.WithStores(nil, nil) // 无存储

	ctx := domain.NewAddContext(context.Background(), "agent", "user", "session")
	ctx.Messages = domain.Messages{
		{Role: "user", Content: "测试消息"},
	}

	// 不应 panic
	action.Handle(ctx)

	// 仍然应该提取实体
	assert.Len(t, ctx.Entities, 1)
	assert.NoError(t, ctx.Error())
}

func TestExtractionAction_buildEntities(t *testing.T) {
	mockLLM := NewMockLLMClient()
	mockLLM.GenEmbeddingFunc = func(ctx context.Context, embedderName, text string) ([]float32, error) {
		return []float32{0.1, 0.2, 0.3}, nil
	}

	action := NewExtractionAction()
	action.WithLLMClient(mockLLM)

	ctx := domain.NewAddContext(context.Background(), "agent", "user", "session")

	extracted := []ExtractedEntity{
		{Name: "张三", Type: "person", Description: "一个人"},
		{Name: "北京", Type: "place", Description: "城市"},
	}

	entities := action.buildEntities(ctx, extracted)

	assert.Len(t, entities, 2)
	assert.Equal(t, "张三", entities[0].Name)
	assert.Equal(t, domain.EntityType("person"), entities[0].Type)
	assert.Equal(t, "agent", entities[0].AgentID)
	assert.Equal(t, "user", entities[0].UserID)
	assert.Len(t, entities[0].Embedding, 3) // 有 embedding

	assert.Equal(t, "北京", entities[1].Name)
	assert.Equal(t, domain.EntityType("place"), entities[1].Type)
}

func TestExtractionAction_buildEntities_EmbeddingError(t *testing.T) {
	mockLLM := NewMockLLMClient()
	mockLLM.GenEmbeddingFunc = func(ctx context.Context, embedderName, text string) ([]float32, error) {
		return nil, errors.New("embedding error")
	}

	action := NewExtractionAction()
	action.WithLLMClient(mockLLM)

	ctx := domain.NewAddContext(context.Background(), "agent", "user", "session")

	extracted := []ExtractedEntity{
		{Name: "张三", Type: "person"},
	}

	// 不应 panic，应该继续处理
	entities := action.buildEntities(ctx, extracted)

	assert.Len(t, entities, 1)
	assert.Equal(t, "张三", entities[0].Name)
	assert.Nil(t, entities[0].Embedding) // embedding 失败时为空
}

func TestExtractionAction_buildEdges(t *testing.T) {
	mockLLM := NewMockLLMClient()
	mockLLM.GenEmbeddingFunc = func(ctx context.Context, embedderName, text string) ([]float32, error) {
		return []float32{0.1, 0.2, 0.3}, nil
	}

	action := NewExtractionAction()
	action.WithLLMClient(mockLLM)

	ctx := domain.NewAddContext(context.Background(), "agent", "user", "session")
	ctx.Episodes = []domain.Episode{{ID: "ep_1"}, {ID: "ep_2"}}

	entities := []domain.Entity{
		{ID: "ent_1", Name: "张三"},
		{ID: "ent_2", Name: "李四"},
	}

	relations := []ExtractedRelation{
		{Subject: "张三", Predicate: "认识", Object: "李四", Fact: "张三认识李四"},
	}

	edges := action.buildEdges(ctx, relations, entities)

	assert.Len(t, edges, 1)
	assert.Equal(t, "ent_1", edges[0].SourceID)
	assert.Equal(t, "ent_2", edges[0].TargetID)
	assert.Equal(t, "认识", edges[0].Relation)
	assert.Equal(t, "张三认识李四", edges[0].Fact)
	assert.Len(t, edges[0].EpisodeIDs, 2)
	assert.Len(t, edges[0].Embedding, 3)
}

func TestExtractionAction_buildEdges_UnknownEntity(t *testing.T) {
	mockLLM := NewMockLLMClient()
	action := NewExtractionAction()
	action.WithLLMClient(mockLLM)

	ctx := domain.NewAddContext(context.Background(), "agent", "user", "session")

	entities := []domain.Entity{
		{ID: "ent_1", Name: "张三"},
	}

	// 关系引用了不存在的实体
	relations := []ExtractedRelation{
		{Subject: "张三", Predicate: "认识", Object: "不存在的人", Fact: "测试"},
	}

	edges := action.buildEdges(ctx, relations, entities)

	// 应该跳过无效关系
	assert.Len(t, edges, 0)
}

func TestExtractionAction_buildEdges_EmbeddingError(t *testing.T) {
	mockLLM := NewMockLLMClient()
	mockLLM.GenEmbeddingFunc = func(ctx context.Context, embedderName, text string) ([]float32, error) {
		return nil, errors.New("embedding error")
	}

	action := NewExtractionAction()
	action.WithLLMClient(mockLLM)

	ctx := domain.NewAddContext(context.Background(), "agent", "user", "session")

	entities := []domain.Entity{
		{ID: "ent_1", Name: "张三"},
		{ID: "ent_2", Name: "李四"},
	}

	relations := []ExtractedRelation{
		{Subject: "张三", Predicate: "认识", Object: "李四", Fact: "张三认识李四"},
	}

	// 不应 panic
	edges := action.buildEdges(ctx, relations, entities)

	assert.Len(t, edges, 1)
	assert.Nil(t, edges[0].Embedding) // embedding 失败时为空
}

func TestExtractionResult_Types(t *testing.T) {
	// 测试类型定义
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
