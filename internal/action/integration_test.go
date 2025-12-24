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

// Note: These tests use MockPlugin to mock the LLM layer via genkit.
// This tests the complete action logic without external service dependencies.

// TestIntegration_EpisodeAction_EndToEnd 测试 Episode Action 的完整流程
func TestIntegration_EpisodeAction_EndToEnd(t *testing.T) {
	ctx := context.Background()
	helper := NewTestHelper(ctx)
	helper.SetEmbedderVector([]float32{0.1, 0.2, 0.3})
	helper.SetModelJSON(map[string]any{
		"topic": "工作",
	})

	// 创建 mock 存储
	mockStore := NewMockVectorStore()

	t.Run("episode action generates and stores episodes", func(t *testing.T) {
		action := helper.NewEpisodeStorageAction()
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
		action := helper.NewEpisodeStorageAction()
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
		helper := NewTestHelper(ctx)
		helper.SetEmbedderVector([]float32{0.1, 0.2, 0.3})
		helper.SetModelJSON(map[string]any{
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

		action := helper.NewExtractionAction()
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
		helper := NewTestHelper(ctx)

		action := helper.NewExtractionAction()
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
		helper := NewTestHelper(ctx)
		helper.SetEmbedderVector([]float32{0.1, 0.2, 0.3})
		helper.SetModelJSON(map[string]any{
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

		action := helper.NewExtractionAction()
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
		helper := NewTestHelper(ctx)
		helper.SetEmbedderVector([]float32{0.1, 0.2, 0.3})
		helper.SetModelJSON(map[string]any{
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

		action := helper.NewSummaryAction()
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
		helper := NewTestHelper(ctx)

		action := helper.NewSummaryAction()
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
		helper := NewTestHelper(ctx)
		helper.SetEmbedderVector([]float32{0.1, 0.2, 0.3})

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

		action := helper.NewRetrievalAction()
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
		helper := NewTestHelper(ctx)

		mockVector := NewMockVectorStore()
		mockGraph := NewMockGraphStore()

		action := helper.NewRetrievalAction()
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
		helper := NewTestHelper(ctx)
		helper.SetEmbedderVector([]float32{0.1, 0.2, 0.3})

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

		action := helper.NewRetrievalAction()
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
		helper := NewTestHelper(ctx)
		helper.SetEmbedderVector([]float32{0.1, 0.2, 0.3})

		// 使用计数器来根据调用返回不同结果
		callCount := 0
		helper.MockPlugin.SetModelResponse("doubao-pro-32k", func(ctx context.Context, req *ai.ModelRequest) (*ai.ModelResponse, error) {
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
		episodeAction := helper.NewEpisodeStorageAction()
		episodeAction.WithVectorStore(mockVector)

		extractionAction := helper.NewExtractionAction()
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
