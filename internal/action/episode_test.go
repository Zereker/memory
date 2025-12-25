package action

import (
	"context"
	"errors"
	"testing"

	"github.com/firebase/genkit/go/ai"
	"github.com/stretchr/testify/assert"

	"github.com/Zereker/memory/internal/domain"
)

func TestEpisodeStorageAction_Basic(t *testing.T) {
	ctx := context.Background()
	_ = newTestHelper(ctx) // required: initializes genkit registry

	t.Run("Name returns correct name", func(t *testing.T) {
		action := NewEpisodeStorageAction()
		assert.Equal(t, "episode_storage", action.Name())
	})

	t.Run("WithVectorStore returns same instance for chaining", func(t *testing.T) {
		mockStore := NewMockVectorStore()
		action := NewEpisodeStorageAction()
		result := action.WithVectorStore(mockStore)

		assert.Same(t, action, result)
		assert.Equal(t, mockStore, action.vectorStore)
	})

	t.Run("Handle with empty messages produces no episodes", func(t *testing.T) {
		action := NewEpisodeStorageAction()
		action.WithVectorStore(nil)

		addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
		addCtx.Messages = domain.Messages{}

		action.Handle(addCtx)

		assert.Empty(t, addCtx.Episodes, "empty messages should produce no episodes")
		assert.NoError(t, addCtx.Error())
	})
}

func TestEpisodeStorageAction_Handle(t *testing.T) {
	t.Run("WithMessages stores episodes correctly", func(t *testing.T) {
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

		assert.Len(t, mockVector.StoreCalls, 2, "should store 2 episodes")
		assert.Len(t, addCtx.Episodes, 2, "should add 2 episodes to context")
		assert.Equal(t, "旅行", addCtx.Episodes[0].Topic)
		assert.Equal(t, domain.RoleUser, addCtx.Episodes[0].Role)
		assert.Equal(t, "小明", addCtx.Episodes[0].Name)
		assert.NoError(t, addCtx.Error())
	})

	t.Run("ContextCancelled aborts processing", func(t *testing.T) {
		ctx := context.Background()
		_ = newTestHelper(ctx)

		mockVector := NewMockVectorStore()
		action := NewEpisodeStorageAction()
		action.WithVectorStore(mockVector)

		cancelCtx, cancel := context.WithCancel(ctx)
		cancel()

		addCtx := domain.NewAddContext(cancelCtx, "agent", "user", "session")
		addCtx.Messages = domain.Messages{
			{Role: domain.RoleUser, Content: "测试消息"},
		}

		action.Handle(addCtx)

		assert.Error(t, addCtx.Error(), "cancelled context should set error")
		assert.True(t, addCtx.IsAborted(), "should be aborted")
	})

	t.Run("StoreError skips episode without aborting", func(t *testing.T) {
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

		assert.Empty(t, addCtx.Episodes, "store error should skip episode")
		assert.NoError(t, addCtx.Error(), "store error should not abort chain")
	})

	t.Run("ContentEmbeddingFailure skips episode", func(t *testing.T) {
		ctx := context.Background()
		h := newTestHelper(ctx)

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

		assert.Empty(t, addCtx.Episodes, "embedding failure should skip episode")
		assert.NoError(t, addCtx.Error(), "embedding failure should not set error")
	})

	t.Run("TopicGenerationFailure skips episode", func(t *testing.T) {
		ctx := context.Background()
		h := newTestHelper(ctx)
		h.setEmbedderVector([]float32{0.1, 0.2, 0.3})
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

		assert.Empty(t, addCtx.Episodes, "topic generation failure should skip episode")
		assert.NoError(t, addCtx.Error())
	})

	t.Run("TopicEmbeddingFailure skips episode", func(t *testing.T) {
		ctx := context.Background()
		h := newTestHelper(ctx)
		h.setModelJSON(map[string]any{"topic": "test topic"})

		callCount := 0
		h.MockPlugin.SetEmbedderResponse("doubao-embedding-text-240715", func(ctx context.Context, req *ai.EmbedRequest) (*ai.EmbedResponse, error) {
			callCount++
			if callCount == 1 {
				return &ai.EmbedResponse{
					Embeddings: []*ai.Embedding{
						{Embedding: []float32{0.1, 0.2, 0.3}},
					},
				}, nil
			}
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

		assert.Empty(t, addCtx.Episodes, "topic embedding failure should skip episode")
		assert.NoError(t, addCtx.Error())
	})

	t.Run("MultipleMessages with partial failures continues processing", func(t *testing.T) {
		ctx := context.Background()
		h := newTestHelper(ctx)
		h.setModelJSON(map[string]any{"topic": "test topic"})

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

		assert.NoError(t, addCtx.Error(), "partial failures should not abort chain")
		assert.LessOrEqual(t, len(addCtx.Episodes), 3, "some episodes may fail")
	})
}
