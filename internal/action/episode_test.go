package action

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Zereker/memory/internal/domain"
)

func TestEpisodeStorageAction_Name(t *testing.T) {
	ctx := context.Background()
	_ = NewTestHelper(ctx) // Initialize genkit

	action := NewEpisodeStorageAction()
	assert.Equal(t, "episode_storage", action.Name())
}

func TestEpisodeStorageAction_WithVectorStore(t *testing.T) {
	ctx := context.Background()
	_ = NewTestHelper(ctx)

	mockStore := NewMockVectorStore()
	action := NewEpisodeStorageAction()
	result := action.WithVectorStore(mockStore)

	assert.Same(t, action, result)
	assert.Equal(t, mockStore, action.vectorStore)
}

func TestEpisodeStorageAction_Handle_EmptyMessages(t *testing.T) {
	ctx := context.Background()
	helper := NewTestHelper(ctx)

	action := helper.NewEpisodeAction()
	action.WithVectorStore(nil)

	addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
	addCtx.Messages = domain.Messages{} // 空消息

	action.Handle(addCtx)

	assert.Empty(t, addCtx.Episodes)
	assert.NoError(t, addCtx.Error())
}

func TestEpisodeStorageAction_Handle_WithMessages(t *testing.T) {
	ctx := context.Background()
	helper := NewTestHelper(ctx)
	helper.SetEmbedderVector([]float32{0.1, 0.2, 0.3})
	helper.SetModelJSON(map[string]any{"topic": "旅行"})

	mockVector := NewMockVectorStore()

	action := helper.NewEpisodeAction()
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
	helper := NewTestHelper(ctx)
	helper.SetEmbedderVector([]float32{0.1, 0.2, 0.3})
	helper.SetModelJSON(map[string]any{"topic": "测试主题"})

	action := helper.NewEpisodeAction()
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
	helper := NewTestHelper(ctx)

	mockVector := NewMockVectorStore()
	action := helper.NewEpisodeAction()
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
	helper := NewTestHelper(ctx)
	helper.SetEmbedderVector([]float32{0.1, 0.2, 0.3})
	helper.SetModelJSON(map[string]any{"topic": "测试主题"})

	mockVector := NewMockVectorStore()
	mockVector.StoreFunc = func(ctx context.Context, id string, doc map[string]any) error {
		return assert.AnError
	}

	action := helper.NewEpisodeAction()
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
