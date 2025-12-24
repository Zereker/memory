package action

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/firebase/genkit/go/ai"
	"github.com/stretchr/testify/assert"

	"github.com/Zereker/memory/internal/domain"
)

// TestMemoryAdd 测试 Memory.Add 完整流程（使用 MockPlugin）
func TestMemoryAdd(t *testing.T) {
	ctx := context.Background()
	helper := NewTestHelper(ctx)
	helper.SetEmbedderVector([]float32{0.1, 0.2, 0.3})

	// 使用计数器来根据调用返回不同结果
	callCount := 0
	helper.MockPlugin.SetModelResponse("doubao-pro-32k", func(ctx context.Context, req *ai.ModelRequest) (*ai.ModelResponse, error) {
		callCount++
		var response map[string]any
		if callCount <= 2 {
			// Topic 生成
			response = map[string]any{"topic": "个人介绍"}
		} else {
			// Extraction 生成
			response = map[string]any{
				"entities": []map[string]any{
					{"name": "小明", "type": "person", "description": "用户"},
					{"name": "北京", "type": "place", "description": "城市"},
				},
				"relations": []map[string]any{
					{"subject": "小明", "predicate": "住在", "object": "北京", "fact": "小明住在北京"},
				},
			}
		}
		data, _ := json.Marshal(response)
		return &ai.ModelResponse{
			Request: req,
			Message: ai.NewModelTextMessage(string(data)),
		}, nil
	})

	// 创建 mock 存储
	mockVector := NewMockVectorStore()
	mockGraph := NewMockGraphStore()

	// 创建 actions 并注入 mock
	episodeAction := helper.NewEpisodeStorageAction()
	episodeAction.WithVectorStore(mockVector)

	extractionAction := helper.NewExtractionAction()
	extractionAction.WithStores(mockVector, mockGraph)

	c := domain.NewAddContext(ctx, "agent_test", "user_test", "session_test")
	c.Messages = domain.Messages{
		{Role: domain.RoleUser, Name: "小明", Content: "我叫小明，在北京做产品经理，我女朋友叫小红"},
		{Role: domain.RoleAssistant, Name: "AI助手", Content: "你好小明！产品经理是个很有挑战的职业，小红也在北京吗？"},
	}

	// 创建完整的 Add chain
	chain := domain.NewActionChain()
	chain.Use(episodeAction)
	chain.Use(extractionAction)

	chain.Run(c)

	// 验证 Episodes
	assert.Len(t, c.Episodes, 2)

	// 验证 Entities
	assert.Len(t, c.Entities, 2)
	assert.Equal(t, "小明", c.Entities[0].Name)

	// 验证 Edges
	assert.Len(t, c.Edges, 1)
	assert.Equal(t, "小明住在北京", c.Edges[0].Fact)

	// 验证无错误
	assert.NoError(t, c.Error())
}

// mockAddAction 用于测试的 mock action
type mockAddAction struct {
	name    string
	handler func(c *domain.AddContext)
}

func (m *mockAddAction) Name() string {
	return m.name
}

func (m *mockAddAction) Handle(c *domain.AddContext) {
	if m.handler != nil {
		m.handler(c)
	}
}

// TestActionChainAbort 测试 action chain 中断机制
func TestActionChainAbort(t *testing.T) {
	ctx := context.Background()

	executed := []string{}

	// 创建一个会中断的 action
	abortAction := &mockAddAction{
		name: "abort",
		handler: func(c *domain.AddContext) {
			executed = append(executed, "abort")
			c.Abort()
		},
	}

	// 创建一个正常的 action
	normalAction := &mockAddAction{
		name: "normal",
		handler: func(c *domain.AddContext) {
			executed = append(executed, "normal")
			c.Next()
		},
	}

	c := domain.NewAddContext(ctx, "agent_test", "user_test", "session_test")

	chain := domain.NewActionChain()
	chain.Use(abortAction)
	chain.Use(normalAction)
	chain.Run(c)

	// 验证只执行了第一个 action
	assert.Len(t, executed, 1)
	assert.Equal(t, "abort", executed[0])
	assert.True(t, c.IsAborted())
}

// TestActionChainNormal 测试正常的 action chain 执行
func TestActionChainNormal(t *testing.T) {
	ctx := context.Background()

	executed := []string{}

	action1 := &mockAddAction{
		name: "action1",
		handler: func(c *domain.AddContext) {
			executed = append(executed, "action1")
			c.Next()
		},
	}

	action2 := &mockAddAction{
		name: "action2",
		handler: func(c *domain.AddContext) {
			executed = append(executed, "action2")
			c.Next()
		},
	}

	c := domain.NewAddContext(ctx, "agent_test", "user_test", "session_test")

	chain := domain.NewActionChain()
	chain.Use(action1)
	chain.Use(action2)
	chain.Run(c)

	// 验证两个 action 都执行了
	assert.Len(t, executed, 2)
	assert.Equal(t, "action1", executed[0])
	assert.Equal(t, "action2", executed[1])
	assert.False(t, c.IsAborted())
}

// TestActionChainWithError 测试带错误的 action chain
func TestActionChainWithError(t *testing.T) {
	ctx := context.Background()

	executed := []string{}

	errorAction := &mockAddAction{
		name: "error",
		handler: func(c *domain.AddContext) {
			executed = append(executed, "error")
			c.SetError(assert.AnError)
		},
	}

	normalAction := &mockAddAction{
		name: "normal",
		handler: func(c *domain.AddContext) {
			executed = append(executed, "normal")
			c.Next()
		},
	}

	c := domain.NewAddContext(ctx, "agent_test", "user_test", "session_test")

	chain := domain.NewActionChain()
	chain.Use(errorAction)
	chain.Use(normalAction)
	chain.Run(c)

	// 验证只执行了第一个 action（SetError 会中断链）
	assert.Len(t, executed, 1)
	assert.Equal(t, "error", executed[0])
	assert.Error(t, c.Error())
}

// TestNewMemory 测试 Memory 创建
func TestNewMemory(t *testing.T) {
	m := NewMemory()
	assert.NotNil(t, m)
	assert.NotNil(t, m.logger)
}

// TestMemory_Delete 测试删除方法
func TestMemory_Delete(t *testing.T) {
	m := NewMemory()
	err := m.Delete(context.Background(), "test_id")
	assert.NoError(t, err) // 当前实现是空操作
}

// TestInferUserAndAgent 测试用户代理推断
func TestInferUserAndAgent(t *testing.T) {
	t.Run("explicit ids", func(t *testing.T) {
		req := &domain.AddRequest{
			AgentID: "agent_explicit",
			UserID:  "user_explicit",
		}
		userID, agentID := inferUserAndAgent(req)
		assert.Equal(t, "user_explicit", userID)
		assert.Equal(t, "agent_explicit", agentID)
	})

	t.Run("infer from messages", func(t *testing.T) {
		req := &domain.AddRequest{
			Messages: []domain.Message{
				{Role: domain.RoleUser, Name: "张三", Content: "你好"},
				{Role: domain.RoleAssistant, Name: "AI助手", Content: "你好！"},
			},
		}
		userID, agentID := inferUserAndAgent(req)
		assert.Equal(t, "张三", userID)
		assert.Equal(t, "AI助手", agentID)
	})

	t.Run("partial ids", func(t *testing.T) {
		req := &domain.AddRequest{
			AgentID: "agent_explicit",
			Messages: []domain.Message{
				{Role: domain.RoleUser, Name: "张三", Content: "你好"},
			},
		}
		userID, agentID := inferUserAndAgent(req)
		assert.Equal(t, "张三", userID)
		assert.Equal(t, "agent_explicit", agentID)
	})

	t.Run("empty", func(t *testing.T) {
		req := &domain.AddRequest{}
		userID, agentID := inferUserAndAgent(req)
		// When no explicit IDs and no messages, Messages.UserName() returns "user"
		// and Messages.AssistantName() returns "assistant" as defaults
		assert.Equal(t, "user", userID)
		assert.Equal(t, "assistant", agentID)
	})
}

// Note: Memory.Add and Memory.Retrieve require integration tests with external services
// (genkit, OpenSearch, Neo4j) as they create actions internally without mock injection.
// These methods are tested via integration tests or manual testing.
