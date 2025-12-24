package action

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Zereker/memory/internal/domain"
	"github.com/Zereker/memory/pkg/storage"
)

// TestSummaryAction 单元测试
func TestSummaryAction(t *testing.T) {
	newContext := func() *domain.AddContext {
		return domain.NewAddContext(context.Background(), "agent_test", "user_test", "session_summary_test")
	}

	// 创建无存储的 action，用于测试边界情况
	newAction := func() *SummaryAction {
		return &SummaryAction{
			BaseAction: NewBaseAction("summary"),
			store:      nil, // 无存储
		}
	}

	run := func(c *domain.AddContext) {
		chain := domain.NewActionChain()
		chain.Use(newAction())
		chain.Run(c)
	}

	t.Run("NoUserEpisode", func(t *testing.T) {
		c := newContext()
		c.Episodes = []domain.Episode{
			{
				ID:      "ep_test_1",
				Role:    domain.RoleAssistant,
				Name:    "AI助手",
				Content: "你好！",
			},
		}

		run(c)

		if c.Error() != nil {
			t.Fatalf("不应该有错误: %v", c.Error())
		}

		if len(c.Summaries) != 0 {
			t.Error("没有 user episode 时不应该生成 summary")
		}
	})

	t.Run("NoHistoricalEpisode", func(t *testing.T) {
		c := newContext()
		c.Episodes = []domain.Episode{
			{
				ID:             "ep_test_2",
				Role:           domain.RoleUser,
				Name:           "用户",
				Content:        "第一条消息",
				Topic:          "问候",
				TopicEmbedding: make([]float32, 4096),
			},
		}

		run(c)

		if c.Error() != nil {
			t.Fatalf("不应该有错误: %v", c.Error())
		}

		if len(c.Summaries) != 0 {
			t.Error("没有历史 episode 时不应该生成 summary")
		}
	})

	t.Run("MissingTopicEmbedding", func(t *testing.T) {
		c := newContext()
		c.Episodes = []domain.Episode{
			{
				ID:      "ep_test_3",
				Role:    domain.RoleUser,
				Name:    "用户",
				Content: "没有 topic embedding",
				Topic:   "测试",
			},
		}

		run(c)

		if c.Error() != nil {
			t.Fatalf("不应该有错误: %v", c.Error())
		}

		if len(c.Summaries) != 0 {
			t.Error("缺少 topic embedding 时不应该生成 summary")
		}
	})
}

// TestFormatEpisodes 测试 formatEpisodes 方法
func TestFormatEpisodes(t *testing.T) {
	action := NewSummaryAction()

	t.Run("WithNames", func(t *testing.T) {
		episodes := []domain.Episode{
			{Role: domain.RoleUser, Name: "小明", Content: "你好"},
			{Role: domain.RoleAssistant, Name: "AI助手", Content: "你好！有什么可以帮你的？"},
			{Role: domain.RoleUser, Name: "小明", Content: "我想了解一下产品"},
		}

		result := action.formatEpisodes(episodes)
		expected := "小明: 你好\nAI助手: 你好！有什么可以帮你的？\n小明: 我想了解一下产品"

		if result != expected {
			t.Errorf("格式化结果不匹配\n期望: %s\n实际: %s", expected, result)
		}
	})

	t.Run("WithoutNames", func(t *testing.T) {
		episodes := []domain.Episode{
			{Role: domain.RoleUser, Content: "你好"},
			{Role: domain.RoleAssistant, Content: "你好！"},
		}

		result := action.formatEpisodes(episodes)
		expected := "user: 你好\nassistant: 你好！"

		if result != expected {
			t.Errorf("格式化结果不匹配\n期望: %s\n实际: %s", expected, result)
		}
	})

	t.Run("Empty", func(t *testing.T) {
		result := action.formatEpisodes(nil)
		if result != "" {
			t.Errorf("空 episodes 应该返回空字符串，实际: %s", result)
		}
	})
}

// TestSummaryAction_Name 测试 Name 方法
func TestSummaryAction_Name(t *testing.T) {
	action := NewSummaryAction()
	if action.Name() != "summary" {
		t.Errorf("Name() 应该返回 'summary'，实际: %s", action.Name())
	}
}

// TestSummaryAction_Handle_NoStore 测试无存储时的处理
func TestSummaryAction_Handle_NoStore(t *testing.T) {
	// SummaryAction 依赖 store 来加载历史 episodes
	// 当 store 为 nil 时，应该正常跳过
	action := &SummaryAction{
		BaseAction: NewBaseAction("summary"),
		store:      nil,
	}

	c := domain.NewAddContext(context.Background(), "agent", "user", "session")
	c.Episodes = []domain.Episode{
		{
			ID:             "ep_1",
			Role:           domain.RoleUser,
			Content:        "测试消息",
			Topic:          "测试",
			TopicEmbedding: make([]float32, 100),
		},
	}

	nextCalled := false
	chain := domain.NewActionChain()
	chain.Use(action)
	chain.Use(&mockAddAction{
		name: "next",
		handler: func(c *domain.AddContext) {
			nextCalled = true
			c.Next()
		},
	})
	chain.Run(c)

	if !nextCalled {
		t.Error("应该调用 Next")
	}
	if c.Error() != nil {
		t.Errorf("不应该有错误: %v", c.Error())
	}
}

// TestSummaryAction_Handle_TopicSimilar 测试主题相似时跳过
func TestSummaryAction_Handle_TopicSimilar(t *testing.T) {
	action := &SummaryAction{
		BaseAction: NewBaseAction("summary"),
		store:      nil, // 无存储，loadLastUserEpisode 返回 nil
	}

	c := domain.NewAddContext(context.Background(), "agent", "user", "session")
	// 设置阈值
	c.TopicThreshold = 0.8

	// 添加当前 user episode
	c.Episodes = []domain.Episode{
		{
			ID:             "ep_current",
			Role:           domain.RoleUser,
			Content:        "当前消息",
			Topic:          "问候",
			TopicEmbedding: []float32{1.0, 0, 0, 0, 0},
		},
	}

	chain := domain.NewActionChain()
	chain.Use(action)
	chain.Run(c)

	// 因为 store 为 nil，loadLastUserEpisode 返回 nil，所以会跳过
	if len(c.Summaries) != 0 {
		t.Error("无历史 episode 时不应该生成 summary")
	}
}

// TestSummaryAction_WithStore 测试 WithStore 方法
func TestSummaryAction_WithStore(t *testing.T) {
	action := NewSummaryAction()
	mockStore := NewMockVectorStore()

	result := action.WithStore(mockStore)

	assert.Same(t, action, result) // 返回自身，支持链式调用
}

// TestSummaryAction_LoadLastUserEpisode 测试加载历史 Episode
func TestSummaryAction_LoadLastUserEpisode(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockStore := NewMockVectorStore()
		mockStore.SearchFunc = func(ctx context.Context, query storage.SearchQuery) ([]map[string]any, error) {
			return []map[string]any{
				{
					"id":              "ep_old",
					"role":            domain.RoleUser,
					"content":         "历史消息",
					"topic":           "问候",
					"topic_embedding": []float32{0.1, 0.2, 0.3},
				},
			}, nil
		}

		action := &SummaryAction{
			BaseAction: NewBaseAction("summary"),
			store:      mockStore,
		}

		c := domain.NewAddContext(context.Background(), "agent", "user", "session")
		ep, err := action.loadLastUserEpisode(c, "ep_current")

		assert.NoError(t, err)
		assert.NotNil(t, ep)
		assert.Equal(t, "ep_old", ep.ID)
	})

	t.Run("excludes current episode", func(t *testing.T) {
		mockStore := NewMockVectorStore()
		mockStore.SearchFunc = func(ctx context.Context, query storage.SearchQuery) ([]map[string]any, error) {
			return []map[string]any{
				{"id": "ep_current", "role": domain.RoleUser},
				{"id": "ep_old", "role": domain.RoleUser, "topic_embedding": []float32{0.1}},
			}, nil
		}

		action := &SummaryAction{
			BaseAction: NewBaseAction("summary"),
			store:      mockStore,
		}

		c := domain.NewAddContext(context.Background(), "agent", "user", "session")
		ep, err := action.loadLastUserEpisode(c, "ep_current")

		assert.NoError(t, err)
		assert.NotNil(t, ep)
		assert.Equal(t, "ep_old", ep.ID)
	})

	t.Run("search error", func(t *testing.T) {
		mockStore := NewMockVectorStore()
		mockStore.SearchFunc = func(ctx context.Context, query storage.SearchQuery) ([]map[string]any, error) {
			return nil, assert.AnError
		}

		action := &SummaryAction{
			BaseAction: NewBaseAction("summary"),
			store:      mockStore,
		}

		c := domain.NewAddContext(context.Background(), "agent", "user", "session")
		ep, err := action.loadLastUserEpisode(c, "ep_current")

		assert.Error(t, err)
		assert.Nil(t, ep)
	})

	t.Run("no results", func(t *testing.T) {
		mockStore := NewMockVectorStore()
		mockStore.SearchFunc = func(ctx context.Context, query storage.SearchQuery) ([]map[string]any, error) {
			return nil, nil
		}

		action := &SummaryAction{
			BaseAction: NewBaseAction("summary"),
			store:      mockStore,
		}

		c := domain.NewAddContext(context.Background(), "agent", "user", "session")
		ep, err := action.loadLastUserEpisode(c, "ep_current")

		assert.NoError(t, err)
		assert.Nil(t, ep)
	})
}

// TestSummaryAction_LoadEpisodesSinceLastSummary 测试加载待摘要的 Episodes
func TestSummaryAction_LoadEpisodesSinceLastSummary(t *testing.T) {
	t.Run("with previous summary", func(t *testing.T) {
		mockStore := NewMockVectorStore()
		callCount := 0
		mockStore.SearchFunc = func(ctx context.Context, query storage.SearchQuery) ([]map[string]any, error) {
			callCount++
			docType := query.Filters["type"].(string)
			if docType == domain.DocTypeSummary {
				return []map[string]any{
					{"id": "sum_1", "created_at": "2024-01-01T00:00:00Z"},
				}, nil
			}
			// Episode 查询
			return []map[string]any{
				{"id": "ep_1", "role": domain.RoleUser, "content": "消息1"},
				{"id": "ep_2", "role": domain.RoleAssistant, "content": "消息2"},
			}, nil
		}

		action := &SummaryAction{
			BaseAction: NewBaseAction("summary"),
			store:      mockStore,
		}

		c := domain.NewAddContext(context.Background(), "agent", "user", "session")
		episodes, err := action.loadEpisodesSinceLastSummary(c, "ep_current")

		assert.NoError(t, err)
		assert.Len(t, episodes, 2)
		assert.Equal(t, 2, callCount) // summary + episodes
	})

	t.Run("without previous summary", func(t *testing.T) {
		mockStore := NewMockVectorStore()
		mockStore.SearchFunc = func(ctx context.Context, query storage.SearchQuery) ([]map[string]any, error) {
			docType := query.Filters["type"].(string)
			if docType == domain.DocTypeSummary {
				return nil, nil // 无历史摘要
			}
			return []map[string]any{
				{"id": "ep_1", "role": domain.RoleUser, "content": "消息1"},
			}, nil
		}

		action := &SummaryAction{
			BaseAction: NewBaseAction("summary"),
			store:      mockStore,
		}

		c := domain.NewAddContext(context.Background(), "agent", "user", "session")
		episodes, err := action.loadEpisodesSinceLastSummary(c, "ep_current")

		assert.NoError(t, err)
		assert.Len(t, episodes, 1)
	})

	t.Run("excludes current episode", func(t *testing.T) {
		mockStore := NewMockVectorStore()
		mockStore.SearchFunc = func(ctx context.Context, query storage.SearchQuery) ([]map[string]any, error) {
			docType := query.Filters["type"].(string)
			if docType == domain.DocTypeSummary {
				return nil, nil
			}
			return []map[string]any{
				{"id": "ep_current", "role": domain.RoleUser},
				{"id": "ep_old", "role": domain.RoleUser, "content": "旧消息"},
			}, nil
		}

		action := &SummaryAction{
			BaseAction: NewBaseAction("summary"),
			store:      mockStore,
		}

		c := domain.NewAddContext(context.Background(), "agent", "user", "session")
		episodes, err := action.loadEpisodesSinceLastSummary(c, "ep_current")

		assert.NoError(t, err)
		assert.Len(t, episodes, 1)
		assert.Equal(t, "ep_old", episodes[0].ID)
	})

	t.Run("episode search error", func(t *testing.T) {
		mockStore := NewMockVectorStore()
		mockStore.SearchFunc = func(ctx context.Context, query storage.SearchQuery) ([]map[string]any, error) {
			docType := query.Filters["type"].(string)
			if docType == domain.DocTypeSummary {
				return nil, nil
			}
			return nil, assert.AnError
		}

		action := &SummaryAction{
			BaseAction: NewBaseAction("summary"),
			store:      mockStore,
		}

		c := domain.NewAddContext(context.Background(), "agent", "user", "session")
		episodes, err := action.loadEpisodesSinceLastSummary(c, "ep_current")

		assert.Error(t, err)
		assert.Nil(t, episodes)
	})
}

// TestSummaryAction_GenerateAndStoreSummary 测试生成并存储摘要
func TestSummaryAction_GenerateAndStoreSummary(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockStore := NewMockVectorStore()
		mockLLM := NewMockLLMClient()
		mockLLM.GenerateFunc = func(c *domain.AddContext, promptName string, input map[string]any, output any) error {
			if promptName == "summary" {
				result := SummaryResult{Content: "这是一个测试摘要"}
				data, _ := json.Marshal(result)
				return json.Unmarshal(data, output)
			}
			return nil
		}

		action := &SummaryAction{
			BaseAction: NewBaseAction("summary"),
			store:      mockStore,
		}
		action.WithLLMClient(mockLLM)

		c := domain.NewAddContext(context.Background(), "agent", "user", "session")
		episodes := []domain.Episode{
			{ID: "ep_1", Role: domain.RoleUser, Name: "用户", Content: "你好", Topic: "问候"},
			{ID: "ep_2", Role: domain.RoleAssistant, Name: "AI", Content: "你好！"},
		}

		action.generateAndStoreSummary(c, episodes)

		assert.Len(t, c.Summaries, 1)
		assert.Equal(t, "这是一个测试摘要", c.Summaries[0].Content)
		assert.Equal(t, "问候", c.Summaries[0].Topic)
		assert.Len(t, c.Summaries[0].EpisodeIDs, 2)
		assert.Len(t, mockStore.StoreCalls, 1)
	})

	t.Run("empty episodes", func(t *testing.T) {
		action := &SummaryAction{
			BaseAction: NewBaseAction("summary"),
		}

		c := domain.NewAddContext(context.Background(), "agent", "user", "session")

		action.generateAndStoreSummary(c, nil)

		assert.Empty(t, c.Summaries)
	})

	t.Run("generate error", func(t *testing.T) {
		mockLLM := NewMockLLMClient()
		mockLLM.GenerateFunc = func(c *domain.AddContext, promptName string, input map[string]any, output any) error {
			return assert.AnError
		}

		action := &SummaryAction{
			BaseAction: NewBaseAction("summary"),
			store:      NewMockVectorStore(),
		}
		action.WithLLMClient(mockLLM)

		c := domain.NewAddContext(context.Background(), "agent", "user", "session")
		episodes := []domain.Episode{{ID: "ep_1", Content: "test"}}

		action.generateAndStoreSummary(c, episodes)

		assert.Empty(t, c.Summaries)
	})

	t.Run("embedding error continues", func(t *testing.T) {
		mockStore := NewMockVectorStore()
		mockLLM := NewMockLLMClient()
		mockLLM.GenerateFunc = func(c *domain.AddContext, promptName string, input map[string]any, output any) error {
			if promptName == "summary" {
				result := SummaryResult{Content: "摘要内容"}
				data, _ := json.Marshal(result)
				return json.Unmarshal(data, output)
			}
			return nil
		}
		mockLLM.GenEmbeddingFunc = func(ctx context.Context, embedderName, text string) ([]float32, error) {
			return nil, assert.AnError // embedding 失败
		}

		action := &SummaryAction{
			BaseAction: NewBaseAction("summary"),
			store:      mockStore,
		}
		action.WithLLMClient(mockLLM)

		c := domain.NewAddContext(context.Background(), "agent", "user", "session")
		episodes := []domain.Episode{{ID: "ep_1", Content: "test"}}

		action.generateAndStoreSummary(c, episodes)

		// 即使 embedding 失败，也应该存储摘要
		assert.Len(t, c.Summaries, 1)
	})

	t.Run("store error", func(t *testing.T) {
		mockStore := NewMockVectorStore()
		mockStore.StoreFunc = func(ctx context.Context, id string, doc map[string]any) error {
			return assert.AnError
		}
		mockLLM := NewMockLLMClient()
		mockLLM.GenerateFunc = func(c *domain.AddContext, promptName string, input map[string]any, output any) error {
			if promptName == "summary" {
				result := SummaryResult{Content: "摘要"}
				data, _ := json.Marshal(result)
				return json.Unmarshal(data, output)
			}
			return nil
		}

		action := &SummaryAction{
			BaseAction: NewBaseAction("summary"),
			store:      mockStore,
		}
		action.WithLLMClient(mockLLM)

		c := domain.NewAddContext(context.Background(), "agent", "user", "session")
		episodes := []domain.Episode{{ID: "ep_1", Content: "test"}}

		action.generateAndStoreSummary(c, episodes)

		// store 失败时不添加到 context
		assert.Empty(t, c.Summaries)
	})
}

// TestSummaryAction_StoreSummary 测试存储摘要
func TestSummaryAction_StoreSummary(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockStore := NewMockVectorStore()

		action := &SummaryAction{
			BaseAction: NewBaseAction("summary"),
			store:      mockStore,
		}

		c := domain.NewAddContext(context.Background(), "agent", "user", "session")
		summary := domain.Summary{
			ID:         "sum_1",
			AgentID:    "agent",
			UserID:     "user",
			EpisodeIDs: []string{"ep_1"},
			Topic:      "测试",
			Content:    "测试摘要",
		}

		err := action.storeSummary(c, summary)

		assert.NoError(t, err)
		assert.Len(t, mockStore.StoreCalls, 1)
		assert.Equal(t, "sum_1", mockStore.StoreCalls[0].ID)
	})

	t.Run("nil store", func(t *testing.T) {
		action := &SummaryAction{
			BaseAction: NewBaseAction("summary"),
			store:      nil,
		}

		c := domain.NewAddContext(context.Background(), "agent", "user", "session")
		summary := domain.Summary{ID: "sum_1"}

		err := action.storeSummary(c, summary)

		assert.NoError(t, err)
	})
}

// TestSummaryAction_Handle_TopicChange 测试主题变化时生成摘要
func TestSummaryAction_Handle_TopicChange(t *testing.T) {
	mockStore := NewMockVectorStore()
	searchCallCount := 0
	mockStore.SearchFunc = func(ctx context.Context, query storage.SearchQuery) ([]map[string]any, error) {
		searchCallCount++
		// 第一次调用：loadLastUserEpisode
		if searchCallCount == 1 {
			return []map[string]any{
				{
					"id":              "ep_old",
					"role":            domain.RoleUser,
					"topic":           "旧主题",
					"topic_embedding": []float32{0.0, 1.0, 0.0}, // 与当前不同
				},
			}, nil
		}
		// 第二次调用：查找 Summary
		if searchCallCount == 2 {
			return nil, nil // 无历史摘要
		}
		// 第三次调用：loadEpisodesSinceLastSummary
		return []map[string]any{
			{"id": "ep_old", "role": domain.RoleUser, "content": "旧消息"},
		}, nil
	}

	mockLLM := NewMockLLMClient()
	mockLLM.GenerateFunc = func(c *domain.AddContext, promptName string, input map[string]any, output any) error {
		if promptName == "summary" {
			result := SummaryResult{Content: "生成的摘要"}
			data, _ := json.Marshal(result)
			return json.Unmarshal(data, output)
		}
		return nil
	}

	action := &SummaryAction{
		BaseAction: NewBaseAction("summary"),
		store:      mockStore,
	}
	action.WithLLMClient(mockLLM)

	c := domain.NewAddContext(context.Background(), "agent", "user", "session")
	c.TopicThreshold = 0.8
	c.Episodes = []domain.Episode{
		{
			ID:             "ep_current",
			Role:           domain.RoleUser,
			Topic:          "新主题",
			TopicEmbedding: []float32{1.0, 0.0, 0.0}, // 与旧主题不同
		},
	}

	chain := domain.NewActionChain()
	chain.Use(action)
	chain.Run(c)

	assert.Len(t, c.Summaries, 1)
	assert.Equal(t, "生成的摘要", c.Summaries[0].Content)
}

// TestSummaryAction_Handle_TopicSimilarWithStore 测试主题相似时跳过（有存储）
func TestSummaryAction_Handle_TopicSimilarWithStore(t *testing.T) {
	mockStore := NewMockVectorStore()
	mockStore.SearchFunc = func(ctx context.Context, query storage.SearchQuery) ([]map[string]any, error) {
		return []map[string]any{
			{
				"id":              "ep_old",
				"role":            domain.RoleUser,
				"topic":           "问候",
				"topic_embedding": []float32{1.0, 0.0, 0.0}, // 与当前相同
			},
		}, nil
	}

	action := &SummaryAction{
		BaseAction: NewBaseAction("summary"),
		store:      mockStore,
	}

	c := domain.NewAddContext(context.Background(), "agent", "user", "session")
	c.TopicThreshold = 0.8
	c.Episodes = []domain.Episode{
		{
			ID:             "ep_current",
			Role:           domain.RoleUser,
			Topic:          "问候",
			TopicEmbedding: []float32{1.0, 0.0, 0.0}, // 与旧主题相同
		},
	}

	nextCalled := false
	chain := domain.NewActionChain()
	chain.Use(action)
	chain.Use(&mockAddAction{
		name: "next",
		handler: func(c *domain.AddContext) {
			nextCalled = true
			c.Next()
		},
	})
	chain.Run(c)

	assert.True(t, nextCalled)
	assert.Empty(t, c.Summaries) // 主题相似，不生成摘要
}

// TestSummaryAction_Handle_NoEpisodesToSummarize 测试无待摘要 Episodes
func TestSummaryAction_Handle_NoEpisodesToSummarize(t *testing.T) {
	mockStore := NewMockVectorStore()
	searchCallCount := 0
	mockStore.SearchFunc = func(ctx context.Context, query storage.SearchQuery) ([]map[string]any, error) {
		searchCallCount++
		if searchCallCount == 1 {
			return []map[string]any{
				{
					"id":              "ep_old",
					"role":            domain.RoleUser,
					"topic":           "旧主题",
					"topic_embedding": []float32{0.0, 1.0},
				},
			}, nil
		}
		// 无历史摘要，无待摘要 episodes
		return nil, nil
	}

	action := &SummaryAction{
		BaseAction: NewBaseAction("summary"),
		store:      mockStore,
	}

	c := domain.NewAddContext(context.Background(), "agent", "user", "session")
	c.TopicThreshold = 0.8
	c.Episodes = []domain.Episode{
		{
			ID:             "ep_current",
			Role:           domain.RoleUser,
			Topic:          "新主题",
			TopicEmbedding: []float32{1.0, 0.0},
		},
	}

	chain := domain.NewActionChain()
	chain.Use(action)
	chain.Run(c)

	assert.Empty(t, c.Summaries) // 无 episodes 可摘要
}
