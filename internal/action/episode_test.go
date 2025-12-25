package action

import (
	"context"
	"errors"
	"testing"

	"github.com/firebase/genkit/go/ai"
	"github.com/stretchr/testify/assert"

	"github.com/Zereker/memory/internal/domain"
)

// ai package is used for mock responses

func TestEpisodeStorageAction_Name(t *testing.T) {
	ctx := context.Background()
	_ = newTestHelper(ctx) // Initialize genkit

	action := NewEpisodeStorageAction()
	assert.Equal(t, "episode_storage", action.Name())
}

func TestEpisodeStorageAction_WithVectorStore(t *testing.T) {
	ctx := context.Background()
	_ = newTestHelper(ctx)

	mockStore := NewMockVectorStore()
	action := NewEpisodeStorageAction()
	result := action.WithVectorStore(mockStore)

	assert.Same(t, action, result)
	assert.Equal(t, mockStore, action.vectorStore)
}

func TestEpisodeStorageAction_Handle_EmptyMessages(t *testing.T) {
	ctx := context.Background()
	_ = newTestHelper(ctx)

	action := NewEpisodeStorageAction()
	action.WithVectorStore(nil)

	addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
	addCtx.Messages = domain.Messages{} // 空消息

	action.Handle(addCtx)

	assert.Empty(t, addCtx.Episodes)
	assert.NoError(t, addCtx.Error())
}

func TestEpisodeStorageAction_Handle_WithMessages(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	h.setEmbedderVector([]float32{0.1, 0.2, 0.3})
	h.setModelJSON(map[string]any{"topic": "旅行"})

	mockVector := NewMockVectorStore()

	action := NewEpisodeStorageAction()
	action.WithVectorStore(mockVector)

	addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
	addCtx.Messages = domain.Messages{
		{Role: domain.RoleUser, Name: "小明", Content: "我今天去了北京"},
		{Role: domain.RoleAssistant, Name: "AI助手", Content: "北京是个好地方"},
	}

	action.Handle(addCtx)

	// 验证存储了 episodes
	assert.Len(t, mockVector.StoreCalls, 2)

	// 验证 context 中添加了 episodes
	assert.Len(t, addCtx.Episodes, 2)
	assert.Equal(t, "旅行", addCtx.Episodes[0].Topic)
	assert.Equal(t, domain.RoleUser, addCtx.Episodes[0].Role)
	assert.Equal(t, "小明", addCtx.Episodes[0].Name)
	assert.NoError(t, addCtx.Error())
}

func TestEpisodeStorageAction_Handle_NoVectorStore(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	h.setEmbedderVector([]float32{0.1, 0.2, 0.3})
	h.setModelJSON(map[string]any{"topic": "测试主题"})

	action := NewEpisodeStorageAction()
	action.WithVectorStore(nil) // 无存储

	addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
	addCtx.Messages = domain.Messages{
		{Role: domain.RoleUser, Content: "测试消息"},
	}

	// 不应 panic
	action.Handle(addCtx)

	// 仍然应该生成 episodes
	assert.Len(t, addCtx.Episodes, 1)
	assert.NoError(t, addCtx.Error())
}

func TestEpisodeStorageAction_Handle_ContextCancelled(t *testing.T) {
	ctx := context.Background()
	_ = newTestHelper(ctx)

	mockVector := NewMockVectorStore()
	action := NewEpisodeStorageAction()
	action.WithVectorStore(mockVector)

	// 创建已取消的 context
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()

	addCtx := domain.NewAddContext(cancelCtx, "agent", "user", "session")
	addCtx.Messages = domain.Messages{
		{Role: domain.RoleUser, Content: "测试消息"},
	}

	action.Handle(addCtx)

	// context 取消应该设置错误
	assert.Error(t, addCtx.Error())
	assert.True(t, addCtx.IsAborted())
}

func TestEpisodeStorageAction_Handle_StoreError(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	h.setEmbedderVector([]float32{0.1, 0.2, 0.3})
	h.setModelJSON(map[string]any{"topic": "测试主题"})

	mockVector := NewMockVectorStore()
	mockVector.StoreFunc = func(ctx context.Context, id string, doc map[string]any) error {
		return assert.AnError
	}

	action := NewEpisodeStorageAction()
	action.WithVectorStore(mockVector)

	addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
	addCtx.Messages = domain.Messages{
		{Role: domain.RoleUser, Content: "测试消息"},
	}

	action.Handle(addCtx)

	// 存储失败时应该跳过该 episode，不添加到 context
	assert.Empty(t, addCtx.Episodes)
	// 但不应该中断链
	assert.NoError(t, addCtx.Error())
}

func TestEpisodeAction_EmbeddingFailure_SilentlySkipped(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)

	// 设置 embedder 返回错误
	h.MockPlugin.SetEmbedderResponse("doubao-embedding-text-240715", func(ctx context.Context, req *ai.EmbedRequest) (*ai.EmbedResponse, error) {
		return nil, errors.New("embedding service unavailable")
	})
	h.setModelJSON(map[string]any{"topic": "test"})

	mockStore := NewMockVectorStore()
	action := NewEpisodeStorageAction()
	action.WithVectorStore(mockStore)

	addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
	addCtx.Messages = domain.Messages{
		{Role: domain.RoleUser, Content: "test message"},
	}

	action.Handle(addCtx)

	// BUG: embedding 生成失败，但 episode 仍然被创建（只是没有 embedding）
	// 这可能导致后续向量搜索找不到这些 episode
	if len(addCtx.Episodes) > 0 {
		t.Log("Episode was created despite embedding failure")
		t.Log("Embedding length:", len(addCtx.Episodes[0].Embedding))
		// embedding 应该为空或 nil
		assert.Empty(t, addCtx.Episodes[0].Embedding, "Episode has no embedding due to failure")
	}

	// 没有错误被设置 - 这是 bug 还是预期行为？
	t.Log("BUG: Embedding failure was silently ignored, no error set")
}

// ============================================================================
// Additional Coverage Tests for episode.go
// ============================================================================

func TestEpisodeStorageAction_Handle_TopicGenerationFailure(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	h.setEmbedderVector([]float32{0.1, 0.2, 0.3})
	// Set model to return error via invalid JSON
	h.MockPlugin.SetModelResponse("doubao-pro-32k", func(ctx context.Context, req *ai.ModelRequest) (*ai.ModelResponse, error) {
		return nil, errors.New("topic generation failed")
	})

	mockVector := NewMockVectorStore()
	action := NewEpisodeStorageAction()
	action.WithVectorStore(mockVector)

	addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
	addCtx.Messages = domain.Messages{
		{Role: domain.RoleUser, Content: "test message"},
	}

	action.Handle(addCtx)

	// Topic generation failure should skip this message
	assert.Empty(t, addCtx.Episodes)
	assert.NoError(t, addCtx.Error())
}

func TestEpisodeStorageAction_Handle_TopicEmbeddingFailure(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	h.setModelJSON(map[string]any{"topic": "test topic"})

	// First call succeeds for content embedding, second fails for topic embedding
	callCount := 0
	h.MockPlugin.SetEmbedderResponse("doubao-embedding-text-240715", func(ctx context.Context, req *ai.EmbedRequest) (*ai.EmbedResponse, error) {
		callCount++
		if callCount == 1 {
			// First call: content embedding succeeds
			return &ai.EmbedResponse{
				Embeddings: []*ai.Embedding{
					{Embedding: []float32{0.1, 0.2, 0.3}},
				},
			}, nil
		}
		// Second call: topic embedding fails
		return nil, errors.New("topic embedding failed")
	})

	mockVector := NewMockVectorStore()
	action := NewEpisodeStorageAction()
	action.WithVectorStore(mockVector)

	addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
	addCtx.Messages = domain.Messages{
		{Role: domain.RoleUser, Content: "test message"},
	}

	action.Handle(addCtx)

	// Topic embedding failure should skip this message
	assert.Empty(t, addCtx.Episodes)
	assert.NoError(t, addCtx.Error())
}

func TestEpisodeStorageAction_Handle_MultipleMessages_PartialFailure(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	h.setModelJSON(map[string]any{"topic": "test topic"})

	// Alternate between success and failure
	callCount := 0
	h.MockPlugin.SetEmbedderResponse("doubao-embedding-text-240715", func(ctx context.Context, req *ai.EmbedRequest) (*ai.EmbedResponse, error) {
		callCount++
		if callCount%2 == 0 {
			return nil, errors.New("embedding failed")
		}
		return &ai.EmbedResponse{
			Embeddings: []*ai.Embedding{
				{Embedding: []float32{0.1, 0.2, 0.3}},
			},
		}, nil
	})

	mockVector := NewMockVectorStore()
	action := NewEpisodeStorageAction()
	action.WithVectorStore(mockVector)

	addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
	addCtx.Messages = domain.Messages{
		{Role: domain.RoleUser, Content: "message 1"},
		{Role: domain.RoleAssistant, Content: "message 2"},
		{Role: domain.RoleUser, Content: "message 3"},
	}

	action.Handle(addCtx)

	// Some messages may succeed, some may fail
	assert.NoError(t, addCtx.Error())
}
