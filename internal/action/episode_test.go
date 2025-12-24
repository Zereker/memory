package action

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Zereker/memory/internal/domain"
)

func TestEpisodeStorageAction_Name(t *testing.T) {
	action := NewEpisodeStorageAction()
	assert.Equal(t, "episode_storage", action.Name())
}

func TestEpisodeStorageAction_WithVectorStore(t *testing.T) {
	mockVector := NewMockVectorStore()

	action := NewEpisodeStorageAction()
	result := action.WithVectorStore(mockVector)

	assert.Same(t, action, result)
	assert.Equal(t, mockVector, action.vectorStore)
}

func TestEpisodeStorageAction_Handle_EmptyMessages(t *testing.T) {
	mockLLM := NewMockLLMClient()
	mockVector := NewMockVectorStore()

	action := NewEpisodeStorageAction()
	action.WithLLMClient(mockLLM)
	action.WithVectorStore(mockVector)

	ctx := domain.NewAddContext(context.Background(), "agent", "user", "session")
	ctx.Messages = domain.Messages{} // 空消息

	action.Handle(ctx)

	// 应该直接跳过，不调用 LLM
	assert.Empty(t, mockLLM.GenerateCalls)
	assert.Empty(t, mockLLM.GenEmbeddingCalls)
	assert.Empty(t, ctx.Episodes)
	assert.NoError(t, ctx.Error())
}

func TestEpisodeStorageAction_Handle_WithMessages(t *testing.T) {
	mockLLM := NewMockLLMClient()
	mockLLM.GenerateFunc = func(c *domain.AddContext, promptName string, input map[string]any, output any) error {
		if promptName == "topic" {
			result := TopicResult{Topic: "旅行"}
			data, _ := json.Marshal(result)
			return json.Unmarshal(data, output)
		}
		return nil
	}

	mockVector := NewMockVectorStore()

	action := NewEpisodeStorageAction()
	action.WithLLMClient(mockLLM)
	action.WithVectorStore(mockVector)

	ctx := domain.NewAddContext(context.Background(), "agent", "user", "session")
	ctx.Messages = domain.Messages{
		{Role: domain.RoleUser, Name: "小明", Content: "我今天去了北京"},
		{Role: domain.RoleAssistant, Name: "AI助手", Content: "北京是个好地方"},
	}

	action.Handle(ctx)

	// 验证调用了 LLM
	assert.Len(t, mockLLM.GenerateCalls, 2) // 每条消息调用一次 topic
	assert.Equal(t, "topic", mockLLM.GenerateCalls[0].PromptName)

	// 验证生成了 embedding (每条消息生成 content + topic embedding)
	assert.Len(t, mockLLM.GenEmbeddingCalls, 4)

	// 验证存储了 episodes
	assert.Len(t, mockVector.StoreCalls, 2)

	// 验证 context 中添加了 episodes
	assert.Len(t, ctx.Episodes, 2)
	assert.Equal(t, "旅行", ctx.Episodes[0].Topic)
	assert.Equal(t, domain.RoleUser, ctx.Episodes[0].Role)
	assert.Equal(t, "小明", ctx.Episodes[0].Name)
	assert.NoError(t, ctx.Error())
}

func TestEpisodeStorageAction_Handle_EmbeddingError(t *testing.T) {
	mockLLM := NewMockLLMClient()
	mockLLM.GenEmbeddingFunc = func(ctx context.Context, embedderName, text string) ([]float32, error) {
		return nil, errors.New("embedding error")
	}

	mockVector := NewMockVectorStore()

	action := NewEpisodeStorageAction()
	action.WithLLMClient(mockLLM)
	action.WithVectorStore(mockVector)

	ctx := domain.NewAddContext(context.Background(), "agent", "user", "session")
	ctx.Messages = domain.Messages{
		{Role: domain.RoleUser, Content: "测试消息"},
	}

	action.Handle(ctx)

	// embedding 失败时应该跳过该消息，不存储
	assert.Empty(t, mockVector.StoreCalls)
	assert.Empty(t, ctx.Episodes)
	// 但不应该中断链
	assert.NoError(t, ctx.Error())
}

func TestEpisodeStorageAction_Handle_TopicError(t *testing.T) {
	mockLLM := NewMockLLMClient()
	mockLLM.GenerateFunc = func(c *domain.AddContext, promptName string, input map[string]any, output any) error {
		return errors.New("topic generation error")
	}

	mockVector := NewMockVectorStore()

	action := NewEpisodeStorageAction()
	action.WithLLMClient(mockLLM)
	action.WithVectorStore(mockVector)

	ctx := domain.NewAddContext(context.Background(), "agent", "user", "session")
	ctx.Messages = domain.Messages{
		{Role: domain.RoleUser, Content: "测试消息"},
	}

	action.Handle(ctx)

	// topic 生成失败时应该跳过该消息
	assert.Empty(t, mockVector.StoreCalls)
	assert.Empty(t, ctx.Episodes)
	assert.NoError(t, ctx.Error())
}

func TestEpisodeStorageAction_Handle_NoVectorStore(t *testing.T) {
	mockLLM := NewMockLLMClient()
	mockLLM.GenerateFunc = func(c *domain.AddContext, promptName string, input map[string]any, output any) error {
		if promptName == "topic" {
			result := TopicResult{Topic: "测试主题"}
			data, _ := json.Marshal(result)
			return json.Unmarshal(data, output)
		}
		return nil
	}

	action := NewEpisodeStorageAction()
	action.WithLLMClient(mockLLM)
	action.WithVectorStore(nil) // 无存储

	ctx := domain.NewAddContext(context.Background(), "agent", "user", "session")
	ctx.Messages = domain.Messages{
		{Role: domain.RoleUser, Content: "测试消息"},
	}

	// 不应 panic
	action.Handle(ctx)

	// 仍然应该生成 episodes
	assert.Len(t, ctx.Episodes, 1)
	assert.NoError(t, ctx.Error())
}

func TestEpisodeStorageAction_Handle_ContextCancelled(t *testing.T) {
	mockLLM := NewMockLLMClient()
	mockVector := NewMockVectorStore()

	action := NewEpisodeStorageAction()
	action.WithLLMClient(mockLLM)
	action.WithVectorStore(mockVector)

	// 创建已取消的 context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
	addCtx.Messages = domain.Messages{
		{Role: domain.RoleUser, Content: "测试消息"},
	}

	action.Handle(addCtx)

	// context 取消应该设置错误
	assert.Error(t, addCtx.Error())
	assert.True(t, addCtx.IsAborted())
}

func TestEpisodeStorageAction_Handle_StoreError(t *testing.T) {
	mockLLM := NewMockLLMClient()
	mockLLM.GenerateFunc = func(c *domain.AddContext, promptName string, input map[string]any, output any) error {
		if promptName == "topic" {
			result := TopicResult{Topic: "测试主题"}
			data, _ := json.Marshal(result)
			return json.Unmarshal(data, output)
		}
		return nil
	}

	mockVector := NewMockVectorStore()
	mockVector.StoreFunc = func(ctx context.Context, id string, doc map[string]any) error {
		return errors.New("store error")
	}

	action := NewEpisodeStorageAction()
	action.WithLLMClient(mockLLM)
	action.WithVectorStore(mockVector)

	ctx := domain.NewAddContext(context.Background(), "agent", "user", "session")
	ctx.Messages = domain.Messages{
		{Role: domain.RoleUser, Content: "测试消息"},
	}

	action.Handle(ctx)

	// 存储失败时应该跳过该 episode，不添加到 context
	assert.Empty(t, ctx.Episodes)
	// 但不应该中断链
	assert.NoError(t, ctx.Error())
}
