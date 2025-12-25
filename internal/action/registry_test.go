package action

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/firebase/genkit/go/ai"
	"github.com/stretchr/testify/assert"

	"github.com/Zereker/memory/internal/domain"
	"github.com/Zereker/memory/pkg/vector"
)

// TestMemoryAdd 测试 Memory.Add 完整流程（使用 MockPlugin）
func TestMemoryAdd(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	h.setEmbedderVector([]float32{0.1, 0.2, 0.3})

	// 使用计数器来根据调用返回不同结果
	callCount := 0
	h.MockPlugin.SetModelResponse("doubao-pro-32k", func(ctx context.Context, req *ai.ModelRequest) (*ai.ModelResponse, error) {
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
	episodeAction := NewEpisodeStorageAction()
	episodeAction.WithVectorStore(mockVector)

	extractionAction := NewExtractionAction()
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

// ============================================================================
// Integration Tests (complete action flows)
// ============================================================================

// TestIntegration_EpisodeAction_EndToEnd 测试 Episode Action 的完整流程
func TestIntegration_EpisodeAction_EndToEnd(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	h.setEmbedderVector([]float32{0.1, 0.2, 0.3})
	h.setModelJSON(map[string]any{
		"topic": "工作",
	})

	// 创建 mock 存储
	mockStore := NewMockVectorStore()

	t.Run("episode action generates and stores episodes", func(t *testing.T) {
		action := NewEpisodeStorageAction()
		action.WithVectorStore(mockStore)

		c := domain.NewAddContext(ctx, "agent", "user", "session")
		c.Messages = domain.Messages{
			{Role: domain.RoleUser, Name: "张三", Content: "我在北京做产品经理"},
			{Role: domain.RoleAssistant, Name: "AI", Content: "产品经理是个很有挑战的职业！"},
		}

		chain := domain.NewActionChain()
		chain.Use(action)
		chain.Run(c)

		assert.NoError(t, c.Error())
		assert.Len(t, c.Episodes, 2)
		assert.Equal(t, "张三", c.Episodes[0].Name)
		assert.Equal(t, "工作", c.Episodes[0].Topic)
		assert.Equal(t, domain.RoleUser, c.Episodes[0].Role)
		assert.Equal(t, "AI", c.Episodes[1].Name)
		assert.Equal(t, domain.RoleAssistant, c.Episodes[1].Role)

		// 验证存储调用
		assert.Len(t, mockStore.StoreCalls, 2)
	})

	t.Run("episode action handles empty messages", func(t *testing.T) {
		mockStore2 := NewMockVectorStore()
		action := NewEpisodeStorageAction()
		action.WithVectorStore(mockStore2)

		c := domain.NewAddContext(ctx, "agent", "user", "session")
		c.Messages = domain.Messages{} // empty

		chain := domain.NewActionChain()
		chain.Use(action)
		chain.Run(c)

		assert.NoError(t, c.Error())
		assert.Empty(t, c.Episodes)
	})
}

// TestIntegration_ExtractionAction_EndToEnd 测试 Extraction Action 的完整流程
func TestIntegration_ExtractionAction_EndToEnd(t *testing.T) {
	t.Run("extraction action extracts entities and edges", func(t *testing.T) {
		ctx := context.Background()
		h := newTestHelper(ctx)
		h.setEmbedderVector([]float32{0.1, 0.2, 0.3})
		h.setModelJSON(map[string]any{
			"entities": []map[string]any{
				{"name": "张三", "type": "person", "description": "用户"},
				{"name": "北京", "type": "place", "description": "城市"},
			},
			"relations": []map[string]any{
				{"subject": "张三", "predicate": "住在", "object": "北京", "fact": "张三住在北京"},
			},
		})

		// 创建 mock 存储
		mockVector := NewMockVectorStore()
		mockGraph := NewMockGraphStore()

		action := NewExtractionAction()
		action.WithStores(mockVector, mockGraph)

		c := domain.NewAddContext(ctx, "agent", "user", "session")
		c.Messages = domain.Messages{
			{Role: domain.RoleUser, Name: "张三", Content: "我住在北京"},
		}

		chain := domain.NewActionChain()
		chain.Use(action)
		chain.Run(c)

		assert.NoError(t, c.Error())
		assert.Len(t, c.Entities, 2)
		entityNames := []string{c.Entities[0].Name, c.Entities[1].Name}
		assert.Contains(t, entityNames, "张三")
		assert.Contains(t, entityNames, "北京")

		assert.Len(t, c.Edges, 1)
		assert.Equal(t, "张三住在北京", c.Edges[0].Fact)
		assert.Equal(t, "住在", c.Edges[0].Relation)

		// 验证图存储调用（2 entities + 1 relationship）
		assert.Len(t, mockGraph.MergeNodeCalls, 2)
		assert.Len(t, mockGraph.CreateRelationshipCalls, 1)

		// 验证向量存储调用（2 entities + 1 edge）
		assert.Len(t, mockVector.StoreCalls, 3)
	})

	t.Run("extraction action handles empty messages", func(t *testing.T) {
		ctx := context.Background()
		_ = newTestHelper(ctx)

		action := NewExtractionAction()
		action.WithStores(nil, nil)

		c := domain.NewAddContext(ctx, "agent", "user", "session")
		c.Messages = domain.Messages{}

		chain := domain.NewActionChain()
		chain.Use(action)
		chain.Run(c)

		assert.NoError(t, c.Error())
		assert.Empty(t, c.Entities)
		assert.Empty(t, c.Edges)
	})

	t.Run("extraction action skips relations with unknown entities", func(t *testing.T) {
		ctx := context.Background()
		h := newTestHelper(ctx)
		h.setEmbedderVector([]float32{0.1, 0.2, 0.3})
		h.setModelJSON(map[string]any{
			"entities": []map[string]any{
				{"name": "张三", "type": "person", "description": "用户"},
			},
			"relations": []map[string]any{
				// 引用不存在的实体 "上海"
				{"subject": "张三", "predicate": "住在", "object": "上海", "fact": "张三住在上海"},
			},
		})

		mockVector := NewMockVectorStore()
		mockGraph := NewMockGraphStore()

		action := NewExtractionAction()
		action.WithStores(mockVector, mockGraph)

		c := domain.NewAddContext(ctx, "agent", "user", "session")
		c.Messages = domain.Messages{
			{Role: domain.RoleUser, Name: "张三", Content: "我住在上海"},
		}

		chain := domain.NewActionChain()
		chain.Use(action)
		chain.Run(c)

		assert.NoError(t, c.Error())
		assert.Len(t, c.Entities, 1)
		assert.Empty(t, c.Edges) // 关系被跳过因为引用了不存在的实体
	})
}

// TestIntegration_SummaryAction_EndToEnd 测试 Summary Action 的完整流程
func TestIntegration_SummaryAction_EndToEnd(t *testing.T) {
	t.Run("summary action with topic change", func(t *testing.T) {
		ctx := context.Background()
		h := newTestHelper(ctx)
		h.setEmbedderVector([]float32{0.1, 0.2, 0.3})
		h.setModelJSON(map[string]any{
			"content": "用户张三住在北京，是一名产品经理",
		})

		// 创建 mock store 返回历史 episode
		mockStore := NewMockVectorStore()
		searchCount := 0
		mockStore.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
			searchCount++
			docType, _ := query.Filters["type"].(string)
			if docType == domain.DocTypeEpisode && searchCount == 1 {
				// loadLastUserEpisode - 返回不同 topic 的历史 episode
				return []map[string]any{
					{
						"id":              "ep_old",
						"role":            domain.RoleUser,
						"topic":           "旧主题",
						"topic_embedding": []float32{0.0, 1.0, 0.0}, // 与当前不同
					},
				}, nil
			}
			if docType == domain.DocTypeSummary {
				return nil, nil // 无历史摘要
			}
			// loadEpisodesSinceLastSummary
			return []map[string]any{
				{"id": "ep_old", "role": domain.RoleUser, "content": "旧消息"},
			}, nil
		}

		action := NewSummaryAction()
		action.WithStore(mockStore)

		c := domain.NewAddContext(ctx, "agent", "user", "session")
		c.TopicThreshold = 0.8
		c.Episodes = []domain.Episode{
			{
				ID:             "ep_current",
				Role:           domain.RoleUser,
				Topic:          "新主题",
				TopicEmbedding: []float32{1.0, 0.0, 0.0}, // 与旧 topic 不同
			},
		}

		chain := domain.NewActionChain()
		chain.Use(action)
		chain.Run(c)

		assert.NoError(t, c.Error())
		assert.Len(t, c.Summaries, 1)
		assert.Contains(t, c.Summaries[0].Content, "张三")
	})

	t.Run("summary action skips when no user episodes", func(t *testing.T) {
		ctx := context.Background()
		_ = newTestHelper(ctx)

		action := NewSummaryAction()
		action.WithStore(nil)

		c := domain.NewAddContext(ctx, "agent", "user", "session")
		c.Episodes = []domain.Episode{
			{
				ID:   "ep_1",
				Role: domain.RoleAssistant, // 只有 assistant 消息
			},
		}

		chain := domain.NewActionChain()
		chain.Use(action)
		chain.Run(c)

		assert.NoError(t, c.Error())
		assert.Empty(t, c.Summaries)
	})
}

// TestIntegration_RetrievalAction_EndToEnd 测试 Retrieval Action 的完整流程
func TestIntegration_RetrievalAction_EndToEnd(t *testing.T) {
	t.Run("retrieval action retrieves all memory types", func(t *testing.T) {
		ctx := context.Background()
		h := newTestHelper(ctx)
		h.setEmbedderVector([]float32{0.1, 0.2, 0.3})

		mockVector := NewMockVectorStore()
		mockVector.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
			docType, _ := query.Filters["type"].(string)
			switch docType {
			case domain.DocTypeEpisode:
				return []map[string]any{
					{"id": "ep_1", "type": domain.DocTypeEpisode, "role": domain.RoleUser, "content": "我住在北京"},
				}, nil
			case domain.DocTypeSummary:
				return []map[string]any{
					{"id": "sum_1", "type": domain.DocTypeSummary, "topic": "工作", "content": "用户是产品经理"},
				}, nil
			case domain.DocTypeEdge:
				return []map[string]any{
					{"id": "edge_1", "type": domain.DocTypeEdge, "fact": "张三住在北京"},
				}, nil
			case domain.DocTypeEntity:
				return []map[string]any{
					{"id": "ent_1", "type": domain.DocTypeEntity, "name": "张三", "entity_type": "person"},
				}, nil
			}
			return nil, nil
		}

		mockGraph := NewMockGraphStore()

		action := NewRetrievalAction()
		action.WithStores(mockVector, mockGraph)

		req := &domain.RetrieveRequest{
			AgentID: "agent",
			UserID:  "user",
			Query:   "张三住在哪里",
		}
		c := domain.NewRecallContext(ctx, req)

		chain := domain.NewRecallChain()
		chain.Use(action)
		chain.Run(c)

		assert.Len(t, c.Episodes, 1)
		assert.Len(t, c.Summaries, 1)
		assert.Len(t, c.Edges, 1)
		assert.Len(t, c.Entities, 1)
	})

	t.Run("retrieval action handles empty query", func(t *testing.T) {
		ctx := context.Background()
		_ = newTestHelper(ctx)

		mockVector := NewMockVectorStore()
		mockGraph := NewMockGraphStore()

		action := NewRetrievalAction()
		action.WithStores(mockVector, mockGraph)

		req := &domain.RetrieveRequest{
			AgentID: "agent",
			UserID:  "user",
			Query:   "", // 空查询
		}
		c := domain.NewRecallContext(ctx, req)

		chain := domain.NewRecallChain()
		chain.Use(action)
		chain.Run(c)

		// 空查询会导致 embedding 生成失败，action 会跳过
		assert.Empty(t, c.Episodes)
	})

	t.Run("retrieval action with graph traversal", func(t *testing.T) {
		ctx := context.Background()
		h := newTestHelper(ctx)
		h.setEmbedderVector([]float32{0.1, 0.2, 0.3})

		mockVector := NewMockVectorStore()
		mockVector.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
			docType, _ := query.Filters["type"].(string)
			if docType == domain.DocTypeEntity {
				return []map[string]any{
					{"id": "ent_1", "type": domain.DocTypeEntity, "name": "张三", "entity_type": "person"},
				}, nil
			}
			return nil, nil
		}

		mockGraph := NewMockGraphStore()
		mockGraph.TraverseFunc = func(ctx context.Context, startLabel, startKey string, startValue any, relTypes []string, direction string, maxDepth, limit int) ([]map[string]any, error) {
			// 返回关联的实体
			return []map[string]any{
				{"id": "ent_2", "name": "北京", "type": "place"},
			}, nil
		}

		action := NewRetrievalAction()
		action.WithStores(mockVector, mockGraph)

		req := &domain.RetrieveRequest{
			AgentID: "agent",
			UserID:  "user",
			Query:   "张三",
			Options: domain.RetrieveOptions{
				MaxHops: 2, // 启用图遍历
			},
		}
		c := domain.NewRecallContext(ctx, req)

		chain := domain.NewRecallChain()
		chain.Use(action)
		chain.Run(c)

		// 验证图遍历被调用
		assert.GreaterOrEqual(t, len(mockGraph.TraverseCalls), 1)
		assert.Len(t, c.Entities, 2) // 原始 + 遍历结果
	})
}

// TestIntegration_FullPipeline 测试完整的 Add 处理流程
func TestIntegration_FullPipeline(t *testing.T) {
	t.Run("full add pipeline: episode -> extraction", func(t *testing.T) {
		ctx := context.Background()
		h := newTestHelper(ctx)
		h.setEmbedderVector([]float32{0.1, 0.2, 0.3})

		// 使用计数器来根据调用返回不同结果
		callCount := 0
		h.MockPlugin.SetModelResponse("doubao-pro-32k", func(ctx context.Context, req *ai.ModelRequest) (*ai.ModelResponse, error) {
			callCount++
			var response map[string]any
			if callCount == 1 || callCount == 2 {
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
		episodeAction := NewEpisodeStorageAction()
		episodeAction.WithVectorStore(mockVector)

		extractionAction := NewExtractionAction()
		extractionAction.WithStores(mockVector, mockGraph)

		c := domain.NewAddContext(ctx, "agent_test", "user_test", "session_test")
		c.Messages = domain.Messages{
			{Role: domain.RoleUser, Name: "小明", Content: "我叫小明，在北京做产品经理"},
			{Role: domain.RoleAssistant, Name: "AI助手", Content: "你好小明！产品经理是个很有挑战的职业"},
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
		assert.Equal(t, "北京", c.Entities[1].Name)

		// 验证 Edges
		assert.Len(t, c.Edges, 1)
		assert.Equal(t, "小明住在北京", c.Edges[0].Fact)

		// 验证无错误
		assert.NoError(t, c.Error())

		// 验证存储调用
		assert.GreaterOrEqual(t, len(mockVector.StoreCalls), 3) // episodes + entities + edges
		assert.Len(t, mockGraph.MergeNodeCalls, 2)
		assert.Len(t, mockGraph.CreateRelationshipCalls, 1)
	})
}
