package action

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Zereker/memory/internal/domain"
	"github.com/Zereker/memory/pkg/storage"
)

// TestRetrievalAction 单元测试
func TestRetrievalAction(t *testing.T) {
	t.Run("Name", func(t *testing.T) {
		action := NewRetrievalAction()
		assert.Equal(t, "retrieval", action.Name())
	})

	t.Run("DefaultLimit", func(t *testing.T) {
		req := &domain.RetrieveRequest{
			AgentID: "agent_test",
			UserID:  "user_test",
			Query:   "test",
			Limit:   0, // 未设置
		}
		c := domain.NewRecallContext(context.Background(), req)

		// 默认 limit 应该是 10
		assert.Equal(t, 10, c.Limit)
	})

	t.Run("CustomLimit", func(t *testing.T) {
		req := &domain.RetrieveRequest{
			AgentID: "agent_test",
			UserID:  "user_test",
			Query:   "test",
			Limit:   20,
		}
		c := domain.NewRecallContext(context.Background(), req)

		assert.Equal(t, 20, c.Limit)
	})
}

// TestFormatMemoryContext 测试记忆上下文格式化
func TestFormatMemoryContext(t *testing.T) {
	t.Run("AllTypes", func(t *testing.T) {
		req := &domain.RetrieveRequest{
			AgentID: "agent_test",
			UserID:  "user_test",
			Query:   "用户的职业",
		}
		c := domain.NewRecallContext(context.Background(), req)

		c.Episodes = []domain.Episode{
			{Role: domain.RoleUser, Name: "小明", Content: "我在北京做产品经理"},
		}
		c.Entities = []domain.Entity{
			{Name: "小明", Type: domain.EntityTypePerson, Description: "用户"},
			{Name: "北京", Type: domain.EntityTypePlace, Description: "工作地点"},
		}
		c.Edges = []domain.Edge{
			{Fact: "小明是产品经理"},
			{Fact: "小明在北京工作"},
		}
		c.Summaries = []domain.Summary{
			{Topic: "职业", Content: "用户是一名在北京工作的产品经理"},
		}

		result := FormatMemoryContext(c)

		assert.Contains(t, result, "对话摘要")
		assert.Contains(t, result, "用户信息")
		assert.Contains(t, result, "相关对话记录")
		assert.Contains(t, result, "提及的实体")
	})

	t.Run("Empty", func(t *testing.T) {
		req := &domain.RetrieveRequest{
			AgentID: "agent_test",
			UserID:  "user_test",
			Query:   "不存在的信息",
		}
		c := domain.NewRecallContext(context.Background(), req)

		result := FormatMemoryContext(c)

		assert.Contains(t, result, "没有找到")
	})

	t.Run("OnlyEpisodes", func(t *testing.T) {
		req := &domain.RetrieveRequest{
			AgentID: "agent_test",
			UserID:  "user_test",
			Query:   "test",
		}
		c := domain.NewRecallContext(context.Background(), req)
		c.Episodes = []domain.Episode{
			{Role: domain.RoleUser, Name: "用户", Content: "测试内容"},
		}

		result := FormatMemoryContext(c)

		assert.Contains(t, result, "相关对话记录")
		assert.NotContains(t, result, "对话摘要") // 无 Summary
	})

	t.Run("OnlySummaries", func(t *testing.T) {
		req := &domain.RetrieveRequest{
			AgentID: "agent_test",
			UserID:  "user_test",
			Query:   "test",
		}
		c := domain.NewRecallContext(context.Background(), req)
		c.Summaries = []domain.Summary{
			{Topic: "测试", Content: "这是测试摘要"},
		}

		result := FormatMemoryContext(c)

		assert.Contains(t, result, "对话摘要")
		assert.Contains(t, result, "[测试]")
	})

	t.Run("SummaryWithoutTopic", func(t *testing.T) {
		req := &domain.RetrieveRequest{
			AgentID: "agent_test",
			UserID:  "user_test",
			Query:   "test",
		}
		c := domain.NewRecallContext(context.Background(), req)
		c.Summaries = []domain.Summary{
			{Content: "没有主题的摘要"},
		}

		result := FormatMemoryContext(c)

		assert.Contains(t, result, "没有主题的摘要")
		assert.NotContains(t, result, "[]")
	})

	t.Run("EpisodeWithoutName", func(t *testing.T) {
		req := &domain.RetrieveRequest{
			AgentID: "agent_test",
			UserID:  "user_test",
			Query:   "test",
		}
		c := domain.NewRecallContext(context.Background(), req)
		c.Episodes = []domain.Episode{
			{Role: domain.RoleUser, Content: "没有名字的消息"},
		}

		result := FormatMemoryContext(c)

		assert.Contains(t, result, "[user]")
	})

	t.Run("EntityWithDescription", func(t *testing.T) {
		req := &domain.RetrieveRequest{
			AgentID: "agent_test",
			UserID:  "user_test",
			Query:   "test",
		}
		c := domain.NewRecallContext(context.Background(), req)
		c.Entities = []domain.Entity{
			{Name: "小明", Type: domain.EntityTypePerson, Description: "产品经理"},
		}

		result := FormatMemoryContext(c)

		assert.Contains(t, result, "小明: 产品经理")
	})

	t.Run("EntityWithoutDescription", func(t *testing.T) {
		req := &domain.RetrieveRequest{
			AgentID: "agent_test",
			UserID:  "user_test",
			Query:   "test",
		}
		c := domain.NewRecallContext(context.Background(), req)
		c.Entities = []domain.Entity{
			{Name: "北京", Type: domain.EntityTypePlace},
		}

		result := FormatMemoryContext(c)

		assert.Contains(t, result, "北京 (place)")
	})
}

// TestBudgetCalculation 测试 token 预算计算
func TestBudgetCalculation(t *testing.T) {
	t.Run("EstimateTokens", func(t *testing.T) {
		// 中文约 1.5 字符/token
		text := "这是一个测试字符串"
		chars := len([]rune(text))
		expectedTokens := float64(chars) / CharsPerToken

		// 验证常量
		assert.Equal(t, 1.5, CharsPerToken)
		assert.Greater(t, expectedTokens, 0.0)
	})

	t.Run("DefaultMaxTokens", func(t *testing.T) {
		assert.Equal(t, 2000, DefaultMaxTokens)
		assert.Equal(t, 3, DefaultMaxSummaries)
		assert.Equal(t, 10, DefaultMaxEdges)
		assert.Equal(t, 5, DefaultMaxEntities)
		assert.Equal(t, 5, DefaultMaxEpisodes)
	})
}

// TestFormatMemoryContextIntegration 测试完整格式化场景
func TestFormatMemoryContextIntegration(t *testing.T) {
	req := &domain.RetrieveRequest{
		AgentID: "agent_test",
		UserID:  "user_test",
		Query:   "工作和爱好",
	}
	c := domain.NewRecallContext(context.Background(), req)

	// 添加多种类型的数据
	c.Summaries = []domain.Summary{
		{Topic: "工作", Content: "用户在科技公司担任工程师"},
		{Topic: "爱好", Content: "用户喜欢跑步和阅读"},
	}
	c.Entities = []domain.Entity{
		{Name: "张三", Type: domain.EntityTypePerson, Description: "科技公司工程师"},
		{Name: "科技公司", Type: domain.EntityTypeThing},
	}
	c.Edges = []domain.Edge{
		{Fact: "张三在科技公司工作"},
	}
	c.Episodes = []domain.Episode{
		{Role: domain.RoleUser, Name: "张三", Content: "我每天早上去跑步"},
	}

	result := FormatMemoryContext(c)

	// 验证所有部分都存在
	assert.Contains(t, result, "对话摘要")
	assert.Contains(t, result, "[工作]")
	assert.Contains(t, result, "[爱好]")
	assert.Contains(t, result, "用户信息")
	assert.Contains(t, result, "张三在科技公司工作")
	assert.Contains(t, result, "提及的实体")
	assert.Contains(t, result, "相关对话记录")
	assert.Contains(t, result, "[张三]")

	// 验证格式正确
	lines := strings.Split(result, "\n")
	assert.Greater(t, len(lines), 5) // 应该有多行输出
}

// TestResolveLimit 测试限制值解析
func TestResolveLimit(t *testing.T) {
	tests := []struct {
		name         string
		value        int
		defaultValue int
		expected     int
	}{
		{"negative disables", -1, 10, 0},
		{"zero uses default", 0, 10, 10},
		{"positive uses value", 5, 10, 5},
		{"large value", 100, 10, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveLimit(tt.value, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestEstimateTokens 测试 token 估算
func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		minToken int
	}{
		{"empty", "", 0},
		{"chinese", "你好世界", 2}, // 4 chars / 1.5 ≈ 2.67
		{"english", "hello", 3},  // 5 chars / 1.5 ≈ 3.33
		{"mixed", "你好hello", 4}, // 7 chars / 1.5 ≈ 4.67, int truncation gives 4
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := estimateTokens(tt.text)
			assert.GreaterOrEqual(t, tokens, tt.minToken)
		})
	}
}

// TestTruncateByTokens 测试按 token 截断
func TestTruncateByTokens(t *testing.T) {
	t.Run("within budget", func(t *testing.T) {
		items := []string{"hello", "world", "test"}
		result := truncateByTokens(items, 100, func(s string) int { return 5 })
		assert.Len(t, result, 3)
	})

	t.Run("exceeds budget", func(t *testing.T) {
		items := []string{"hello", "world", "test"}
		result := truncateByTokens(items, 12, func(s string) int { return 5 })
		assert.Len(t, result, 2) // 5+5=10, can't fit third
	})

	t.Run("zero budget", func(t *testing.T) {
		items := []string{"hello", "world"}
		result := truncateByTokens(items, 0, func(s string) int { return 5 })
		assert.Nil(t, result)
	})

	t.Run("negative budget", func(t *testing.T) {
		items := []string{"hello"}
		result := truncateByTokens(items, -1, func(s string) int { return 5 })
		assert.Nil(t, result)
	})

	t.Run("empty items", func(t *testing.T) {
		var items []string
		result := truncateByTokens(items, 100, func(s string) int { return 5 })
		assert.Nil(t, result)
	})
}

// TestGetString 测试字符串提取
func TestGetString(t *testing.T) {
	t.Run("exists", func(t *testing.T) {
		m := map[string]any{"key": "value"}
		assert.Equal(t, "value", getString(m, "key"))
	})

	t.Run("not exists", func(t *testing.T) {
		m := map[string]any{"key": "value"}
		assert.Equal(t, "", getString(m, "other"))
	})

	t.Run("wrong type", func(t *testing.T) {
		m := map[string]any{"key": 123}
		assert.Equal(t, "", getString(m, "key"))
	})

	t.Run("nil value", func(t *testing.T) {
		m := map[string]any{"key": nil}
		assert.Equal(t, "", getString(m, "key"))
	})
}

// TestTokenBudget 测试预算管理
func TestTokenBudget(t *testing.T) {
	t.Run("remaining positive", func(t *testing.T) {
		b := &tokenBudget{total: 100, used: 30}
		assert.Equal(t, 70, b.remaining())
	})

	t.Run("remaining zero", func(t *testing.T) {
		b := &tokenBudget{total: 100, used: 100}
		assert.Equal(t, 0, b.remaining())
	})

	t.Run("remaining negative capped", func(t *testing.T) {
		b := &tokenBudget{total: 100, used: 150}
		assert.Equal(t, 0, b.remaining())
	})
}

// TestRetrievalAction_InitBudget 测试预算初始化
func TestRetrievalAction_InitBudget(t *testing.T) {
	action := NewRetrievalAction()

	t.Run("defaults", func(t *testing.T) {
		req := &domain.RetrieveRequest{
			AgentID: "agent",
			UserID:  "user",
			Query:   "test",
		}
		c := domain.NewRecallContext(context.Background(), req)

		budget := action.initBudget(c)

		assert.Equal(t, DefaultMaxTokens, budget.total)
		assert.Equal(t, DefaultMaxSummaries, budget.maxSummaries)
		assert.Equal(t, DefaultMaxEdges, budget.maxEdges)
		assert.Equal(t, DefaultMaxEntities, budget.maxEntities)
		assert.Equal(t, DefaultMaxEpisodes, budget.maxEpisodes)
	})

	t.Run("custom options", func(t *testing.T) {
		req := &domain.RetrieveRequest{
			AgentID: "agent",
			UserID:  "user",
			Query:   "test",
			Options: domain.RetrieveOptions{
				MaxTokens:    5000,
				MaxSummaries: 5,
				MaxEdges:     20,
				MaxEntities:  10,
				MaxEpisodes:  15,
			},
		}
		c := domain.NewRecallContext(context.Background(), req)

		budget := action.initBudget(c)

		assert.Equal(t, 5000, budget.total)
		assert.Equal(t, 5, budget.maxSummaries)
		assert.Equal(t, 20, budget.maxEdges)
		assert.Equal(t, 10, budget.maxEntities)
		assert.Equal(t, 15, budget.maxEpisodes)
	})

	t.Run("disabled types", func(t *testing.T) {
		req := &domain.RetrieveRequest{
			AgentID: "agent",
			UserID:  "user",
			Query:   "test",
			Options: domain.RetrieveOptions{
				MaxSummaries: -1, // disabled
				MaxEdges:     -1,
				MaxEntities:  -1,
				MaxEpisodes:  -1,
			},
		}
		c := domain.NewRecallContext(context.Background(), req)

		budget := action.initBudget(c)

		assert.Equal(t, 0, budget.maxSummaries)
		assert.Equal(t, 0, budget.maxEdges)
		assert.Equal(t, 0, budget.maxEntities)
		assert.Equal(t, 0, budget.maxEpisodes)
	})
}

// TestRetrievalAction_EstimateTokens 测试 token 估算方法
func TestRetrievalAction_EstimateTokens(t *testing.T) {
	action := NewRetrievalAction()

	t.Run("summary tokens", func(t *testing.T) {
		summaries := []domain.Summary{
			{Topic: "工作", Content: "用户是工程师"},
			{Topic: "爱好", Content: "喜欢编程"},
		}
		tokens := action.estimateSummaryTokens(summaries)
		assert.Greater(t, tokens, 0)
	})

	t.Run("edge tokens", func(t *testing.T) {
		edges := []domain.Edge{
			{Fact: "张三在北京工作"},
			{Fact: "张三喜欢编程"},
		}
		tokens := action.estimateEdgeTokens(edges)
		assert.Greater(t, tokens, 0)
	})

	t.Run("entity tokens", func(t *testing.T) {
		entities := []domain.Entity{
			{Name: "张三", Description: "工程师"},
			{Name: "北京", Description: "城市"},
		}
		tokens := action.estimateEntityTokens(entities)
		assert.Greater(t, tokens, 0)
	})

	t.Run("episode tokens", func(t *testing.T) {
		episodes := []domain.Episode{
			{Content: "我在北京工作"},
			{Content: "我喜欢编程"},
		}
		tokens := action.estimateEpisodeTokens(episodes)
		assert.Greater(t, tokens, 0)
	})

	t.Run("empty slices", func(t *testing.T) {
		assert.Equal(t, 0, action.estimateSummaryTokens(nil))
		assert.Equal(t, 0, action.estimateEdgeTokens(nil))
		assert.Equal(t, 0, action.estimateEntityTokens(nil))
		assert.Equal(t, 0, action.estimateEpisodeTokens(nil))
	})
}

// TestRetrievalAction_Truncate 测试截断方法
func TestRetrievalAction_Truncate(t *testing.T) {
	action := NewRetrievalAction()

	t.Run("truncateSummaries", func(t *testing.T) {
		req := &domain.RetrieveRequest{AgentID: "a", UserID: "u", Query: "q"}
		c := domain.NewRecallContext(context.Background(), req)
		c.Summaries = []domain.Summary{
			{ID: "1"}, {ID: "2"}, {ID: "3"}, {ID: "4"}, {ID: "5"},
		}

		action.truncateSummaries(c, 3)

		assert.Len(t, c.Summaries, 3)
	})

	t.Run("truncateEdges", func(t *testing.T) {
		req := &domain.RetrieveRequest{AgentID: "a", UserID: "u", Query: "q"}
		c := domain.NewRecallContext(context.Background(), req)
		c.Edges = []domain.Edge{
			{ID: "1", Fact: "fact1"},
			{ID: "2", Fact: "fact2"},
			{ID: "3", Fact: "fact3"},
		}

		action.truncateEdges(c, 2, 1000)

		assert.Len(t, c.Edges, 2)
	})

	t.Run("truncateEntities", func(t *testing.T) {
		req := &domain.RetrieveRequest{AgentID: "a", UserID: "u", Query: "q"}
		c := domain.NewRecallContext(context.Background(), req)
		c.Entities = []domain.Entity{
			{ID: "1", Name: "n1", Description: "d1"},
			{ID: "2", Name: "n2", Description: "d2"},
			{ID: "3", Name: "n3", Description: "d3"},
		}

		action.truncateEntities(c, 2, 1000)

		assert.Len(t, c.Entities, 2)
	})

	t.Run("truncateEpisodes", func(t *testing.T) {
		req := &domain.RetrieveRequest{AgentID: "a", UserID: "u", Query: "q"}
		c := domain.NewRecallContext(context.Background(), req)
		c.Episodes = []domain.Episode{
			{ID: "1", Content: "c1"},
			{ID: "2", Content: "c2"},
			{ID: "3", Content: "c3"},
		}

		action.truncateEpisodes(c, 2, 1000)

		assert.Len(t, c.Episodes, 2)
	})
}

// TestRetrievalAction_FilterCoveredEpisodes 测试过滤已覆盖的 Episodes
func TestRetrievalAction_FilterCoveredEpisodes(t *testing.T) {
	action := NewRetrievalAction()

	t.Run("filters covered episodes", func(t *testing.T) {
		req := &domain.RetrieveRequest{AgentID: "a", UserID: "u", Query: "q"}
		c := domain.NewRecallContext(context.Background(), req)

		c.Summaries = []domain.Summary{
			{EpisodeIDs: []string{"ep_1", "ep_2"}},
		}
		c.Episodes = []domain.Episode{
			{ID: "ep_1", Content: "covered"},
			{ID: "ep_2", Content: "covered"},
			{ID: "ep_3", Content: "not covered"},
		}

		action.filterCoveredEpisodes(c)

		assert.Len(t, c.Episodes, 1)
		assert.Equal(t, "ep_3", c.Episodes[0].ID)
	})

	t.Run("no summaries", func(t *testing.T) {
		req := &domain.RetrieveRequest{AgentID: "a", UserID: "u", Query: "q"}
		c := domain.NewRecallContext(context.Background(), req)

		c.Episodes = []domain.Episode{
			{ID: "ep_1", Content: "content1"},
			{ID: "ep_2", Content: "content2"},
		}

		action.filterCoveredEpisodes(c)

		assert.Len(t, c.Episodes, 2)
	})

	t.Run("no episodes", func(t *testing.T) {
		req := &domain.RetrieveRequest{AgentID: "a", UserID: "u", Query: "q"}
		c := domain.NewRecallContext(context.Background(), req)

		c.Summaries = []domain.Summary{
			{EpisodeIDs: []string{"ep_1"}},
		}

		action.filterCoveredEpisodes(c)

		assert.Len(t, c.Episodes, 0)
	})
}

// TestRetrievalAction_HandleRecall_NoVectorStore 测试无存储时的处理
func TestRetrievalAction_HandleRecall_NoVectorStore(t *testing.T) {
	// 创建一个没有存储的 action（通过修改内部字段）
	action := &RetrievalAction{
		BaseAction:  NewBaseAction("retrieval"),
		vectorStore: nil,
		graphStore:  nil,
	}

	// 注入 mock LLM
	mockLLM := NewMockLLMClient()
	action.WithLLMClient(mockLLM)

	req := &domain.RetrieveRequest{
		AgentID: "agent",
		UserID:  "user",
		Query:   "test query",
	}
	c := domain.NewRecallContext(context.Background(), req)

	nextCalled := false
	chain := domain.NewRecallChain()
	chain.Use(action)
	chain.Use(&mockRecallAction{
		handler: func(c *domain.RecallContext) {
			nextCalled = true
			c.Next()
		},
	})
	chain.Run(c)

	// 应该调用 Next，即使没有结果
	assert.True(t, nextCalled)
	assert.Empty(t, c.Episodes)
	assert.Empty(t, c.Summaries)
}

// mockRecallAction 用于测试的 mock RecallAction
type mockRecallAction struct {
	handler func(c *domain.RecallContext)
}

func (m *mockRecallAction) Name() string {
	return "mock_recall"
}

func (m *mockRecallAction) HandleRecall(c *domain.RecallContext) {
	if m.handler != nil {
		m.handler(c)
	}
}

// TestRetrievalAction_SearchEpisodes 测试 Episode 搜索
func TestRetrievalAction_SearchEpisodes(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockVector := NewMockVectorStore()
		mockVector.SearchFunc = func(ctx context.Context, query storage.SearchQuery) ([]map[string]any, error) {
			return []map[string]any{
				{
					"id":      "ep_1",
					"type":    domain.DocTypeEpisode,
					"role":    domain.RoleUser,
					"content": "测试内容",
					"_score":  0.95,
				},
			}, nil
		}

		action := &RetrievalAction{
			BaseAction:  NewBaseAction("retrieval"),
			vectorStore: mockVector,
		}

		req := &domain.RetrieveRequest{AgentID: "a", UserID: "u", Query: "q"}
		c := domain.NewRecallContext(context.Background(), req)
		c.Embedding = []float32{0.1, 0.2}

		action.searchEpisodes(c)

		assert.Len(t, c.Episodes, 1)
		assert.Equal(t, "ep_1", c.Episodes[0].ID)
		assert.Equal(t, 0.95, c.Episodes[0].Score)
	})

	t.Run("search error", func(t *testing.T) {
		mockVector := NewMockVectorStore()
		mockVector.SearchFunc = func(ctx context.Context, query storage.SearchQuery) ([]map[string]any, error) {
			return nil, assert.AnError
		}

		action := &RetrievalAction{
			BaseAction:  NewBaseAction("retrieval"),
			vectorStore: mockVector,
		}

		req := &domain.RetrieveRequest{AgentID: "a", UserID: "u", Query: "q"}
		c := domain.NewRecallContext(context.Background(), req)
		c.Embedding = []float32{0.1, 0.2}

		action.searchEpisodes(c)

		assert.Empty(t, c.Episodes)
	})

	t.Run("nil store", func(t *testing.T) {
		action := &RetrievalAction{
			BaseAction:  NewBaseAction("retrieval"),
			vectorStore: nil,
		}

		req := &domain.RetrieveRequest{AgentID: "a", UserID: "u", Query: "q"}
		c := domain.NewRecallContext(context.Background(), req)

		action.searchEpisodes(c)

		assert.Empty(t, c.Episodes)
	})
}

// TestRetrievalAction_SearchSummaries 测试 Summary 搜索
func TestRetrievalAction_SearchSummaries(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockVector := NewMockVectorStore()
		mockVector.SearchFunc = func(ctx context.Context, query storage.SearchQuery) ([]map[string]any, error) {
			return []map[string]any{
				{
					"id":          "sum_1",
					"type":        domain.DocTypeSummary,
					"topic":       "工作",
					"content":     "用户是工程师",
					"episode_ids": []string{"ep_1", "ep_2"},
					"_score":      0.88,
				},
			}, nil
		}

		action := &RetrievalAction{
			BaseAction:  NewBaseAction("retrieval"),
			vectorStore: mockVector,
		}

		req := &domain.RetrieveRequest{AgentID: "a", UserID: "u", Query: "q"}
		c := domain.NewRecallContext(context.Background(), req)
		c.Embedding = []float32{0.1, 0.2}

		action.searchSummaries(c)

		assert.Len(t, c.Summaries, 1)
		assert.Equal(t, "sum_1", c.Summaries[0].ID)
		assert.Equal(t, 0.88, c.Summaries[0].Score)
	})

	t.Run("search error", func(t *testing.T) {
		mockVector := NewMockVectorStore()
		mockVector.SearchFunc = func(ctx context.Context, query storage.SearchQuery) ([]map[string]any, error) {
			return nil, assert.AnError
		}

		action := &RetrievalAction{
			BaseAction:  NewBaseAction("retrieval"),
			vectorStore: mockVector,
		}

		req := &domain.RetrieveRequest{AgentID: "a", UserID: "u", Query: "q"}
		c := domain.NewRecallContext(context.Background(), req)

		action.searchSummaries(c)

		assert.Empty(t, c.Summaries)
	})
}

// TestRetrievalAction_SearchEdges 测试 Edge 搜索
func TestRetrievalAction_SearchEdges(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockVector := NewMockVectorStore()
		mockVector.SearchFunc = func(ctx context.Context, query storage.SearchQuery) ([]map[string]any, error) {
			return []map[string]any{
				{
					"id":      "edge_1",
					"type":    domain.DocTypeEdge,
					"subject": "小明",
					"object":  "北京",
					"fact":    "小明住在北京",
					"_score":  0.92,
				},
			}, nil
		}

		action := &RetrievalAction{
			BaseAction:  NewBaseAction("retrieval"),
			vectorStore: mockVector,
		}

		req := &domain.RetrieveRequest{AgentID: "a", UserID: "u", Query: "q"}
		c := domain.NewRecallContext(context.Background(), req)
		c.Embedding = []float32{0.1, 0.2}

		action.searchEdges(c)

		assert.Len(t, c.Edges, 1)
		assert.Equal(t, "edge_1", c.Edges[0].ID)
		assert.Equal(t, 0.92, c.Edges[0].Score)
	})

	t.Run("search error", func(t *testing.T) {
		mockVector := NewMockVectorStore()
		mockVector.SearchFunc = func(ctx context.Context, query storage.SearchQuery) ([]map[string]any, error) {
			return nil, assert.AnError
		}

		action := &RetrievalAction{
			BaseAction:  NewBaseAction("retrieval"),
			vectorStore: mockVector,
		}

		req := &domain.RetrieveRequest{AgentID: "a", UserID: "u", Query: "q"}
		c := domain.NewRecallContext(context.Background(), req)

		action.searchEdges(c)

		assert.Empty(t, c.Edges)
	})
}

// TestRetrievalAction_SearchEntities 测试 Entity 搜索
func TestRetrievalAction_SearchEntities(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockVector := NewMockVectorStore()
		mockVector.SearchFunc = func(ctx context.Context, query storage.SearchQuery) ([]map[string]any, error) {
			return []map[string]any{
				{
					"id":          "ent_1",
					"type":        domain.DocTypeEntity,
					"name":        "小明",
					"entity_type": "person",
					"description": "用户",
					"_score":      0.85,
				},
			}, nil
		}

		action := &RetrievalAction{
			BaseAction:  NewBaseAction("retrieval"),
			vectorStore: mockVector,
		}

		req := &domain.RetrieveRequest{AgentID: "a", UserID: "u", Query: "q"}
		c := domain.NewRecallContext(context.Background(), req)
		c.Embedding = []float32{0.1, 0.2}

		action.searchEntities(c)

		assert.Len(t, c.Entities, 1)
		assert.Equal(t, "ent_1", c.Entities[0].ID)
		assert.Equal(t, 0.85, c.Entities[0].Score)
	})

	t.Run("search error", func(t *testing.T) {
		mockVector := NewMockVectorStore()
		mockVector.SearchFunc = func(ctx context.Context, query storage.SearchQuery) ([]map[string]any, error) {
			return nil, assert.AnError
		}

		action := &RetrievalAction{
			BaseAction:  NewBaseAction("retrieval"),
			vectorStore: mockVector,
		}

		req := &domain.RetrieveRequest{AgentID: "a", UserID: "u", Query: "q"}
		c := domain.NewRecallContext(context.Background(), req)

		action.searchEntities(c)

		assert.Empty(t, c.Entities)
	})
}

// TestRetrievalAction_ExpandByGraphTraversal 测试图遍历扩展
func TestRetrievalAction_ExpandByGraphTraversal(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockGraph := NewMockGraphStore()
		mockGraph.TraverseFunc = func(ctx context.Context, startLabel, startKey string, startValue any, relTypes []string, direction string, maxDepth, limit int) ([]map[string]any, error) {
			return []map[string]any{
				{
					"id":          "ent_2",
					"agent_id":    "a",
					"user_id":     "u",
					"name":        "小红",
					"type":        "person",
					"description": "小明的朋友",
				},
			}, nil
		}

		action := &RetrievalAction{
			BaseAction: NewBaseAction("retrieval"),
			graphStore: mockGraph,
		}

		req := &domain.RetrieveRequest{
			AgentID: "a",
			UserID:  "u",
			Query:   "q",
			Options: domain.RetrieveOptions{MaxHops: 2},
		}
		c := domain.NewRecallContext(context.Background(), req)
		c.Entities = []domain.Entity{
			{ID: "ent_1", Name: "小明"},
		}

		action.expandByGraphTraversal(c)

		assert.Len(t, c.Entities, 2)
		assert.Equal(t, "ent_2", c.Entities[1].ID)
	})

	t.Run("skip duplicates", func(t *testing.T) {
		mockGraph := NewMockGraphStore()
		mockGraph.TraverseFunc = func(ctx context.Context, startLabel, startKey string, startValue any, relTypes []string, direction string, maxDepth, limit int) ([]map[string]any, error) {
			return []map[string]any{
				{"id": "ent_1", "name": "小明"}, // 重复
				{"id": "ent_2", "name": "小红"},
			}, nil
		}

		action := &RetrievalAction{
			BaseAction: NewBaseAction("retrieval"),
			graphStore: mockGraph,
		}

		req := &domain.RetrieveRequest{
			AgentID: "a",
			UserID:  "u",
			Query:   "q",
			Options: domain.RetrieveOptions{MaxHops: 2},
		}
		c := domain.NewRecallContext(context.Background(), req)
		c.Entities = []domain.Entity{
			{ID: "ent_1", Name: "小明"},
		}

		action.expandByGraphTraversal(c)

		assert.Len(t, c.Entities, 2) // ent_1 不重复添加
	})

	t.Run("traverse error", func(t *testing.T) {
		mockGraph := NewMockGraphStore()
		mockGraph.TraverseFunc = func(ctx context.Context, startLabel, startKey string, startValue any, relTypes []string, direction string, maxDepth, limit int) ([]map[string]any, error) {
			return nil, assert.AnError
		}

		action := &RetrievalAction{
			BaseAction: NewBaseAction("retrieval"),
			graphStore: mockGraph,
		}

		req := &domain.RetrieveRequest{
			AgentID: "a",
			UserID:  "u",
			Query:   "q",
			Options: domain.RetrieveOptions{MaxHops: 2},
		}
		c := domain.NewRecallContext(context.Background(), req)
		c.Entities = []domain.Entity{
			{ID: "ent_1", Name: "小明"},
		}

		action.expandByGraphTraversal(c)

		// 错误时保持原有 entities
		assert.Len(t, c.Entities, 1)
	})

	t.Run("nil graph store", func(t *testing.T) {
		action := &RetrievalAction{
			BaseAction: NewBaseAction("retrieval"),
			graphStore: nil,
		}

		req := &domain.RetrieveRequest{AgentID: "a", UserID: "u", Query: "q"}
		c := domain.NewRecallContext(context.Background(), req)
		c.Entities = []domain.Entity{{ID: "ent_1"}}

		action.expandByGraphTraversal(c)

		assert.Len(t, c.Entities, 1) // 无变化
	})
}

// TestRetrievalAction_HandleRecall_WithStores 测试完整的 HandleRecall 流程
func TestRetrievalAction_HandleRecall_WithStores(t *testing.T) {
	t.Run("full flow", func(t *testing.T) {
		mockVector := NewMockVectorStore()
		searchCallCount := 0
		mockVector.SearchFunc = func(ctx context.Context, query storage.SearchQuery) ([]map[string]any, error) {
			searchCallCount++
			docType := query.Filters["type"].(string)
			switch docType {
			case domain.DocTypeSummary:
				return []map[string]any{
					{"id": "sum_1", "type": domain.DocTypeSummary, "topic": "工作", "content": "用户是工程师"},
				}, nil
			case domain.DocTypeEdge:
				return []map[string]any{
					{"id": "edge_1", "type": domain.DocTypeEdge, "fact": "用户住在北京"},
				}, nil
			case domain.DocTypeEntity:
				return []map[string]any{
					{"id": "ent_1", "type": domain.DocTypeEntity, "name": "用户", "entity_type": "person"},
				}, nil
			case domain.DocTypeEpisode:
				return []map[string]any{
					{"id": "ep_1", "type": domain.DocTypeEpisode, "role": domain.RoleUser, "content": "测试"},
				}, nil
			}
			return nil, nil
		}

		mockLLM := NewMockLLMClient()

		action := &RetrievalAction{
			BaseAction:  NewBaseAction("retrieval"),
			vectorStore: mockVector,
			graphStore:  nil,
		}
		action.WithLLMClient(mockLLM)

		req := &domain.RetrieveRequest{
			AgentID: "a",
			UserID:  "u",
			Query:   "用户的工作",
		}
		c := domain.NewRecallContext(context.Background(), req)

		chain := domain.NewRecallChain()
		chain.Use(action)
		chain.Run(c)

		assert.Len(t, c.Summaries, 1)
		assert.Len(t, c.Edges, 1)
		assert.Len(t, c.Entities, 1)
		assert.Len(t, c.Episodes, 1)
		assert.Equal(t, 4, searchCallCount) // 4种类型各搜索一次
	})

	t.Run("embedding error", func(t *testing.T) {
		mockLLM := NewMockLLMClient()
		mockLLM.GenEmbeddingFunc = func(ctx context.Context, embedderName, text string) ([]float32, error) {
			return nil, assert.AnError
		}

		action := &RetrievalAction{
			BaseAction:  NewBaseAction("retrieval"),
			vectorStore: NewMockVectorStore(),
		}
		action.WithLLMClient(mockLLM)

		req := &domain.RetrieveRequest{AgentID: "a", UserID: "u", Query: "test"}
		c := domain.NewRecallContext(context.Background(), req)

		nextCalled := false
		chain := domain.NewRecallChain()
		chain.Use(action)
		chain.Use(&mockRecallAction{handler: func(c *domain.RecallContext) {
			nextCalled = true
			c.Next()
		}})
		chain.Run(c)

		assert.True(t, nextCalled) // 即使有错误也调用 Next
		assert.Empty(t, c.Summaries)
	})

	t.Run("with graph traversal", func(t *testing.T) {
		mockVector := NewMockVectorStore()
		mockVector.SearchFunc = func(ctx context.Context, query storage.SearchQuery) ([]map[string]any, error) {
			docType := query.Filters["type"].(string)
			if docType == domain.DocTypeEntity {
				return []map[string]any{
					{"id": "ent_1", "type": domain.DocTypeEntity, "name": "小明", "entity_type": "person"},
				}, nil
			}
			return nil, nil
		}

		mockGraph := NewMockGraphStore()
		mockGraph.TraverseFunc = func(ctx context.Context, startLabel, startKey string, startValue any, relTypes []string, direction string, maxDepth, limit int) ([]map[string]any, error) {
			return []map[string]any{
				{"id": "ent_2", "name": "小红", "type": "person"},
			}, nil
		}

		mockLLM := NewMockLLMClient()

		action := &RetrievalAction{
			BaseAction:  NewBaseAction("retrieval"),
			vectorStore: mockVector,
			graphStore:  mockGraph,
		}
		action.WithLLMClient(mockLLM)

		req := &domain.RetrieveRequest{
			AgentID: "a",
			UserID:  "u",
			Query:   "test",
			Options: domain.RetrieveOptions{MaxHops: 2},
		}
		c := domain.NewRecallContext(context.Background(), req)

		chain := domain.NewRecallChain()
		chain.Use(action)
		chain.Run(c)

		assert.Len(t, c.Entities, 2) // 原始 + 图遍历扩展
	})

	t.Run("disabled types", func(t *testing.T) {
		mockVector := NewMockVectorStore()
		searchTypes := []string{}
		mockVector.SearchFunc = func(ctx context.Context, query storage.SearchQuery) ([]map[string]any, error) {
			searchTypes = append(searchTypes, query.Filters["type"].(string))
			return nil, nil
		}

		mockLLM := NewMockLLMClient()

		action := &RetrievalAction{
			BaseAction:  NewBaseAction("retrieval"),
			vectorStore: mockVector,
		}
		action.WithLLMClient(mockLLM)

		req := &domain.RetrieveRequest{
			AgentID: "a",
			UserID:  "u",
			Query:   "test",
			Options: domain.RetrieveOptions{
				MaxSummaries: -1, // 禁用
				MaxEdges:     -1,
				MaxEntities:  -1,
				MaxEpisodes:  -1,
			},
		}
		c := domain.NewRecallContext(context.Background(), req)

		chain := domain.NewRecallChain()
		chain.Use(action)
		chain.Run(c)

		assert.Empty(t, searchTypes) // 所有类型都被禁用，不应有搜索
	})
}

// TestRetrievalAction_WithStores 测试 WithStores 方法
func TestRetrievalAction_WithStores(t *testing.T) {
	mockVector := NewMockVectorStore()
	mockGraph := NewMockGraphStore()

	action := NewRetrievalAction()
	result := action.WithStores(mockVector, mockGraph)

	assert.Same(t, action, result) // 返回自身，支持链式调用
}
