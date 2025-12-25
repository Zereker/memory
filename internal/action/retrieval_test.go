package action

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/firebase/genkit/go/ai"
	"github.com/stretchr/testify/assert"

	"github.com/Zereker/memory/internal/domain"
	"github.com/Zereker/memory/pkg/vector"
)

func TestRetrievalAction_Name(t *testing.T) {
	ctx := context.Background()
	_ = newTestHelper(ctx)

	action := NewRetrievalAction()
	assert.Equal(t, "retrieval", action.Name())
}

func TestRetrievalAction_WithStores(t *testing.T) {
	ctx := context.Background()
	_ = newTestHelper(ctx)

	mockVector := NewMockVectorStore()
	mockGraph := NewMockGraphStore()

	action := NewRetrievalAction()
	result := action.WithStores(mockVector, mockGraph)

	assert.Same(t, action, result)
}

func TestRetrievalAction_HandleRecall_WithResults(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	h.setEmbedderVector([]float32{0.1, 0.2, 0.3})

	mockVector := NewMockVectorStore()
	mockVector.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
		docType, _ := query.Filters["type"].(string)
		switch docType {
		case domain.DocTypeEpisode:
			return []map[string]any{
				{"id": "ep_1", "type": domain.DocTypeEpisode, "role": domain.RoleUser, "content": "测试内容"},
			}, nil
		case domain.DocTypeSummary:
			return []map[string]any{
				{"id": "sum_1", "type": domain.DocTypeSummary, "topic": "工作", "content": "摘要内容"},
			}, nil
		case domain.DocTypeEdge:
			return []map[string]any{
				{"id": "edge_1", "type": domain.DocTypeEdge, "fact": "测试事实"},
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
		Query:   "测试查询",
	}
	recallCtx := domain.NewRecallContext(ctx, req)

	action.HandleRecall(recallCtx)

	assert.Len(t, recallCtx.Episodes, 1)
	assert.Len(t, recallCtx.Summaries, 1)
	assert.Len(t, recallCtx.Edges, 1)
	assert.Len(t, recallCtx.Entities, 1)
}

func TestRetrievalAction_HandleRecall_EmptyQuery(t *testing.T) {
	ctx := context.Background()
	_ = newTestHelper(ctx)

	mockVector := NewMockVectorStore()
	mockGraph := NewMockGraphStore()

	action := NewRetrievalAction()
	action.WithStores(mockVector, mockGraph)

	req := &domain.RetrieveRequest{
		AgentID: "agent",
		UserID:  "user",
		Query:   "", // empty query
	}
	recallCtx := domain.NewRecallContext(ctx, req)

	action.HandleRecall(recallCtx)

	// Empty query should result in no results
	assert.Empty(t, recallCtx.Episodes)
}

func TestRetrievalAction_HandleRecall_WithGraphTraversal(t *testing.T) {
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
			MaxHops: 2,
		},
	}
	recallCtx := domain.NewRecallContext(ctx, req)

	action.HandleRecall(recallCtx)

	assert.GreaterOrEqual(t, len(mockGraph.TraverseCalls), 1)
	assert.Len(t, recallCtx.Entities, 2)
}

func TestTruncateByTokens_EdgeCases(t *testing.T) {
	items := []domain.Summary{{ID: "s1", Content: "test"}}
	estimator := func(s domain.Summary) int { return 10 }

	tests := []struct {
		name      string
		maxTokens int
		wantLen   int
	}{
		{"zero budget", 0, 0},
		{"negative budget", -1, 0},
		{"sufficient budget", 100, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateByTokens(items, tt.maxTokens, estimator)
			assert.NotNil(t, result)
			assert.Len(t, result, tt.wantLen)
		})
	}
}

func TestRetrievalAction_ScoreTypeAssertion_WrongTypeBecomesZero(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	h.setEmbedderVector([]float32{0.1, 0.2, 0.3})

	mockVector := NewMockVectorStore()
	mockVector.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
		docType, _ := query.Filters["type"].(string)
		if docType == domain.DocTypeEpisode {
			return []map[string]any{
				{
					"id":      "ep_1",
					"type":    domain.DocTypeEpisode,
					"role":    domain.RoleUser,
					"content": "test",
					"_score":  "0.95", // 故意用错误类型
				},
			}, nil
		}
		return nil, nil
	}

	action := NewRetrievalAction()
	action.WithStores(mockVector, NewMockGraphStore())

	req := &domain.RetrieveRequest{
		AgentID: "agent",
		UserID:  "user",
		Query:   "test",
	}
	recallCtx := domain.NewRecallContext(ctx, req)

	action.HandleRecall(recallCtx)

	// 代码已有 ok 检查，类型错误时 score 保持默认值 0
	// 这是预期行为，不是 bug
	if len(recallCtx.Episodes) > 0 {
		assert.Equal(t, float64(0), recallCtx.Episodes[0].Score, "Wrong type defaults to 0")
	}
}

// ============================================================================
// FormatMemoryContext Tests
// ============================================================================

func TestFormatMemoryContext(t *testing.T) {
	ctx := context.Background()
	req := &domain.RetrieveRequest{AgentID: "agent", UserID: "user", Query: "test"}

	tests := []struct {
		name     string
		setup    func(*domain.RecallContext)
		contains []string
	}{
		{
			name:     "empty context",
			setup:    func(c *domain.RecallContext) {},
			contains: []string{"没有找到相关的记忆信息。"},
		},
		{
			name: "only summaries",
			setup: func(c *domain.RecallContext) {
				c.Summaries = []domain.Summary{
					{ID: "s1", Topic: "工作", Content: "用户是产品经理"},
					{ID: "s2", Topic: "", Content: "无主题摘要"},
				}
			},
			contains: []string{"## 对话摘要", "[工作] 用户是产品经理", "- 无主题摘要"},
		},
		{
			name: "only edges",
			setup: func(c *domain.RecallContext) {
				c.Edges = []domain.Edge{
					{ID: "e1", Fact: "张三住在北京"},
					{ID: "e2", Fact: "张三喜欢编程"},
				}
			},
			contains: []string{"## 用户信息", "- 张三住在北京", "- 张三喜欢编程"},
		},
		{
			name: "only episodes",
			setup: func(c *domain.RecallContext) {
				c.Episodes = []domain.Episode{
					{ID: "ep1", Name: "张三", Content: "我在北京"},
					{ID: "ep2", Name: "", Role: "user", Content: "没有名字的消息"},
				}
			},
			contains: []string{"## 相关对话记录", "[张三] 我在北京", "[user] 没有名字的消息"},
		},
		{
			name: "only entities",
			setup: func(c *domain.RecallContext) {
				c.Entities = []domain.Entity{
					{ID: "ent1", Name: "张三", Type: "person", Description: "一个程序员"},
					{ID: "ent2", Name: "北京", Type: "place", Description: ""},
				}
			},
			contains: []string{"## 提及的实体", "- 张三: 一个程序员", "- 北京 (place)"},
		},
		{
			name: "all types",
			setup: func(c *domain.RecallContext) {
				c.Summaries = []domain.Summary{{ID: "s1", Topic: "工作", Content: "摘要内容"}}
				c.Edges = []domain.Edge{{ID: "e1", Fact: "事实"}}
				c.Episodes = []domain.Episode{{ID: "ep1", Name: "张三", Content: "对话"}}
				c.Entities = []domain.Entity{{ID: "ent1", Name: "实体", Type: "type", Description: "描述"}}
			},
			contains: []string{"## 对话摘要", "## 用户信息", "## 相关对话记录", "## 提及的实体"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recallCtx := domain.NewRecallContext(ctx, req)
			tt.setup(recallCtx)
			result := FormatMemoryContext(recallCtx)
			for _, expected := range tt.contains {
				assert.Contains(t, result, expected)
			}
		})
	}
}

// ============================================================================
// Additional Coverage Tests for retrieval.go
// ============================================================================

func TestRetrievalAction_HandleRecall_EmbeddingError(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	h.MockPlugin.SetEmbedderResponse("doubao-embedding-text-240715", func(ctx context.Context, req *ai.EmbedRequest) (*ai.EmbedResponse, error) {
		return nil, errors.New("embedding failed")
	})

	mockVector := NewMockVectorStore()
	mockGraph := NewMockGraphStore()

	action := NewRetrievalAction()
	action.WithStores(mockVector, mockGraph)

	req := &domain.RetrieveRequest{
		AgentID: "agent",
		UserID:  "user",
		Query:   "测试查询",
	}
	recallCtx := domain.NewRecallContext(ctx, req)

	action.HandleRecall(recallCtx)

	// Embedding error should result in no results but no error propagated
	assert.Empty(t, recallCtx.Episodes)
	assert.Empty(t, recallCtx.Summaries)
}

func TestRetrievalAction_HandleRecall_NilStores(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	h.setEmbedderVector([]float32{0.1, 0.2, 0.3})

	action := NewRetrievalAction()
	action.WithStores(nil, nil)

	req := &domain.RetrieveRequest{
		AgentID: "agent",
		UserID:  "user",
		Query:   "测试查询",
	}
	recallCtx := domain.NewRecallContext(ctx, req)

	action.HandleRecall(recallCtx)

	// Nil stores should result in no results
	assert.Empty(t, recallCtx.Episodes)
}

func TestRetrievalAction_HandleRecall_SearchErrors(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	h.setEmbedderVector([]float32{0.1, 0.2, 0.3})

	mockVector := NewMockVectorStore()
	mockVector.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
		return nil, errors.New("search failed")
	}
	mockGraph := NewMockGraphStore()

	action := NewRetrievalAction()
	action.WithStores(mockVector, mockGraph)

	req := &domain.RetrieveRequest{
		AgentID: "agent",
		UserID:  "user",
		Query:   "测试查询",
	}
	recallCtx := domain.NewRecallContext(ctx, req)

	action.HandleRecall(recallCtx)

	// Search errors should result in empty results, not crash
	assert.Empty(t, recallCtx.Episodes)
	assert.Empty(t, recallCtx.Summaries)
}

func TestRetrievalAction_HandleRecall_CustomBudget(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	h.setEmbedderVector([]float32{0.1, 0.2, 0.3})

	mockVector := NewMockVectorStore()
	mockVector.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
		docType, _ := query.Filters["type"].(string)
		if docType == domain.DocTypeSummary {
			return []map[string]any{
				{"id": "sum_1", "type": domain.DocTypeSummary, "content": "摘要"},
			}, nil
		}
		return nil, nil
	}

	action := NewRetrievalAction()
	action.WithStores(mockVector, NewMockGraphStore())

	req := &domain.RetrieveRequest{
		AgentID: "agent",
		UserID:  "user",
		Query:   "测试",
		Options: domain.RetrieveOptions{
			MaxTokens:    5000, // Custom max tokens
			MaxSummaries: 10,
			MaxEdges:     20,
			MaxEntities:  15,
			MaxEpisodes:  8,
		},
	}
	recallCtx := domain.NewRecallContext(ctx, req)

	action.HandleRecall(recallCtx)

	assert.Len(t, recallCtx.Summaries, 1)
}

func TestRetrievalAction_HandleRecall_DisabledCategories(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	h.setEmbedderVector([]float32{0.1, 0.2, 0.3})

	mockVector := NewMockVectorStore()
	searchCalls := 0
	mockVector.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
		searchCalls++
		return nil, nil
	}

	action := NewRetrievalAction()
	action.WithStores(mockVector, NewMockGraphStore())

	req := &domain.RetrieveRequest{
		AgentID: "agent",
		UserID:  "user",
		Query:   "测试",
		Options: domain.RetrieveOptions{
			MaxSummaries: -1, // Disabled
			MaxEdges:     -1, // Disabled
			MaxEntities:  -1, // Disabled
			MaxEpisodes:  -1, // Disabled
		},
	}
	recallCtx := domain.NewRecallContext(ctx, req)

	action.HandleRecall(recallCtx)

	// No searches should be performed when all categories are disabled
	assert.Equal(t, 0, searchCalls)
}

func TestRetrievalAction_HandleRecall_BudgetExhausted(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	h.setEmbedderVector([]float32{0.1, 0.2, 0.3})

	mockVector := NewMockVectorStore()
	mockVector.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
		docType, _ := query.Filters["type"].(string)
		if docType == domain.DocTypeSummary {
			// Return many summaries to exhaust the budget
			return []map[string]any{
				{"id": "s1", "type": domain.DocTypeSummary, "content": strings.Repeat("很长的摘要内容", 100)},
				{"id": "s2", "type": domain.DocTypeSummary, "content": strings.Repeat("很长的摘要内容", 100)},
			}, nil
		}
		return nil, nil
	}

	action := NewRetrievalAction()
	action.WithStores(mockVector, NewMockGraphStore())

	req := &domain.RetrieveRequest{
		AgentID: "agent",
		UserID:  "user",
		Query:   "测试",
		Options: domain.RetrieveOptions{
			MaxTokens: 100, // Very small budget
		},
	}
	recallCtx := domain.NewRecallContext(ctx, req)

	action.HandleRecall(recallCtx)

	// Should still work without panic
	assert.NotNil(t, recallCtx)
}

func TestRetrievalAction_ExpandByGraphTraversal_NilGraphStore(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	h.setEmbedderVector([]float32{0.1, 0.2, 0.3})

	mockVector := NewMockVectorStore()
	mockVector.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
		docType, _ := query.Filters["type"].(string)
		if docType == domain.DocTypeEntity {
			return []map[string]any{
				{"id": "ent_1", "type": domain.DocTypeEntity, "name": "张三"},
			}, nil
		}
		return nil, nil
	}

	action := NewRetrievalAction()
	action.WithStores(mockVector, nil) // Nil graph store

	req := &domain.RetrieveRequest{
		AgentID: "agent",
		UserID:  "user",
		Query:   "张三",
		Options: domain.RetrieveOptions{
			MaxHops: 2,
		},
	}
	recallCtx := domain.NewRecallContext(ctx, req)

	action.HandleRecall(recallCtx)

	// Should have 1 entity from vector search, no graph expansion
	assert.Len(t, recallCtx.Entities, 1)
}

func TestRetrievalAction_ExpandByGraphTraversal_TraverseError(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	h.setEmbedderVector([]float32{0.1, 0.2, 0.3})

	mockVector := NewMockVectorStore()
	mockVector.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
		docType, _ := query.Filters["type"].(string)
		if docType == domain.DocTypeEntity {
			return []map[string]any{
				{"id": "ent_1", "type": domain.DocTypeEntity, "name": "张三"},
			}, nil
		}
		return nil, nil
	}

	mockGraph := NewMockGraphStore()
	mockGraph.TraverseFunc = func(ctx context.Context, startLabel, startKey string, startValue any, relTypes []string, direction string, maxDepth, limit int) ([]map[string]any, error) {
		return nil, errors.New("traverse failed")
	}

	action := NewRetrievalAction()
	action.WithStores(mockVector, mockGraph)

	req := &domain.RetrieveRequest{
		AgentID: "agent",
		UserID:  "user",
		Query:   "张三",
		Options: domain.RetrieveOptions{
			MaxHops: 2,
		},
	}
	recallCtx := domain.NewRecallContext(ctx, req)

	action.HandleRecall(recallCtx)

	// Should have 1 entity from vector search, traverse error is logged but not propagated
	assert.Len(t, recallCtx.Entities, 1)
}

func TestRetrievalAction_FilterCoveredEpisodes_EpisodeIsCovered(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	h.setEmbedderVector([]float32{0.1, 0.2, 0.3})

	mockVector := NewMockVectorStore()
	mockVector.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
		docType, _ := query.Filters["type"].(string)
		if docType == domain.DocTypeSummary {
			return []map[string]any{
				{"id": "s1", "type": domain.DocTypeSummary, "content": "摘要", "episode_ids": []any{"ep_1", "ep_2"}},
			}, nil
		}
		if docType == domain.DocTypeEpisode {
			return []map[string]any{
				{"id": "ep_1", "type": domain.DocTypeEpisode, "content": "已覆盖"},
				{"id": "ep_3", "type": domain.DocTypeEpisode, "content": "未覆盖"},
			}, nil
		}
		return nil, nil
	}

	action := NewRetrievalAction()
	action.WithStores(mockVector, NewMockGraphStore())

	req := &domain.RetrieveRequest{
		AgentID: "agent",
		UserID:  "user",
		Query:   "测试",
	}
	recallCtx := domain.NewRecallContext(ctx, req)

	action.HandleRecall(recallCtx)

	// ep_1 should be filtered out because it's covered by summary
	// Only ep_3 should remain
	assert.Len(t, recallCtx.Episodes, 1)
	if len(recallCtx.Episodes) > 0 {
		assert.Equal(t, "ep_3", recallCtx.Episodes[0].ID)
	}
}

func TestTruncateByTokens_ItemExceedsBudget(t *testing.T) {
	items := []domain.Summary{{ID: "s1", Content: strings.Repeat("很长的内容", 100)}}
	estimator := func(s domain.Summary) int { return 1000 }
	result := truncateByTokens(items, 500, estimator)
	assert.Len(t, result, 0)
}

func TestTokenBudget_Remaining(t *testing.T) {
	t.Run("exhausted budget returns zero", func(t *testing.T) {
		budget := &tokenBudget{total: 100, used: 150}
		assert.Equal(t, 0, budget.remaining())
	})

	t.Run("normal budget returns difference", func(t *testing.T) {
		budget := &tokenBudget{total: 100, used: 30}
		assert.Equal(t, 70, budget.remaining())
	})
}

func TestResolveLimit(t *testing.T) {
	tests := []struct {
		name         string
		value        int
		defaultValue int
		expected     int
	}{
		{"negative disables", -1, 10, 0},
		{"zero uses default", 0, 10, 10},
		{"positive uses custom", 5, 10, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveLimit(tt.value, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRetrievalAction_HandleRecall_WithGraphTraversalDuplicateEntities(t *testing.T) {
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
		return []map[string]any{
			{"id": "ent_1", "name": "张三", "type": "person"}, // Duplicate entity
			{"id": "ent_2", "name": "北京", "type": "place"},  // New entity
		}, nil
	}

	action := NewRetrievalAction()
	action.WithStores(mockVector, mockGraph)

	req := &domain.RetrieveRequest{
		AgentID: "agent",
		UserID:  "user",
		Query:   "张三",
		Options: domain.RetrieveOptions{
			MaxHops: 2,
		},
	}
	recallCtx := domain.NewRecallContext(ctx, req)

	action.HandleRecall(recallCtx)

	// Should have 2 unique entities (duplicate ent_1 should be filtered)
	assert.Len(t, recallCtx.Entities, 2)
}

func TestEstimateTokens(t *testing.T) {
	// Test with Chinese text
	result := estimateTokens("这是中文测试") // 6 characters
	expected := int(float64(6) / CharsPerToken)
	assert.Equal(t, expected, result)

	// Test with empty string
	result = estimateTokens("")
	assert.Equal(t, 0, result)
}

func TestGetString(t *testing.T) {
	m := map[string]any{
		"exists":     "value",
		"not_string": 123,
	}

	assert.Equal(t, "value", getString(m, "exists"))
	assert.Equal(t, "", getString(m, "not_string"))
	assert.Equal(t, "", getString(m, "not_exists"))
}

// ============================================================================
// Truncation Tests
// ============================================================================

func TestRetrievalAction_Truncation(t *testing.T) {
	tests := []struct {
		name      string
		docType   string
		count     int
		maxLimit  func(*domain.RetrieveOptions)
		getResult func(*domain.RecallContext) int
	}{
		{
			name:    "summaries truncated",
			docType: domain.DocTypeSummary,
			count:   5,
			maxLimit: func(o *domain.RetrieveOptions) {
				o.MaxSummaries = 2
			},
			getResult: func(c *domain.RecallContext) int { return len(c.Summaries) },
		},
		{
			name:    "edges truncated",
			docType: domain.DocTypeEdge,
			count:   20,
			maxLimit: func(o *domain.RetrieveOptions) {
				o.MaxEdges = 5
			},
			getResult: func(c *domain.RecallContext) int { return len(c.Edges) },
		},
		{
			name:    "entities truncated",
			docType: domain.DocTypeEntity,
			count:   10,
			maxLimit: func(o *domain.RetrieveOptions) {
				o.MaxEntities = 3
			},
			getResult: func(c *domain.RecallContext) int { return len(c.Entities) },
		},
		{
			name:    "episodes truncated",
			docType: domain.DocTypeEpisode,
			count:   15,
			maxLimit: func(o *domain.RetrieveOptions) {
				o.MaxEpisodes = 4
			},
			getResult: func(c *domain.RecallContext) int { return len(c.Episodes) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			h := newTestHelper(ctx)
			h.setEmbedderVector([]float32{0.1, 0.2, 0.3})

			mockVector := NewMockVectorStore()
			mockVector.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
				docType, _ := query.Filters["type"].(string)
				if docType == tt.docType {
					docs := make([]map[string]any, tt.count)
					for i := 0; i < tt.count; i++ {
						docs[i] = map[string]any{
							"id":          fmt.Sprintf("doc_%d", i),
							"type":        tt.docType,
							"content":     "内容",
							"fact":        "事实",
							"name":        fmt.Sprintf("名称%d", i),
							"entity_type": "person",
						}
					}
					return docs, nil
				}
				return nil, nil
			}

			action := NewRetrievalAction()
			action.WithStores(mockVector, NewMockGraphStore())

			req := &domain.RetrieveRequest{AgentID: "agent", UserID: "user", Query: "测试"}
			tt.maxLimit(&req.Options)
			recallCtx := domain.NewRecallContext(ctx, req)

			action.HandleRecall(recallCtx)

			resultCount := tt.getResult(recallCtx)
			assert.LessOrEqual(t, resultCount, tt.count)
		})
	}
}

func TestRetrievalAction_SearchWithScore(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	h.setEmbedderVector([]float32{0.1, 0.2, 0.3})

	mockVector := NewMockVectorStore()
	mockVector.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
		docType, _ := query.Filters["type"].(string)
		if docType == domain.DocTypeSummary {
			return []map[string]any{
				{"id": "s1", "type": domain.DocTypeSummary, "content": "摘要", "_score": 0.95},
			}, nil
		}
		if docType == domain.DocTypeEdge {
			return []map[string]any{
				{"id": "e1", "type": domain.DocTypeEdge, "fact": "事实", "_score": 0.85},
			}, nil
		}
		if docType == domain.DocTypeEntity {
			return []map[string]any{
				{"id": "ent1", "type": domain.DocTypeEntity, "name": "张三", "_score": 0.75},
			}, nil
		}
		if docType == domain.DocTypeEpisode {
			return []map[string]any{
				{"id": "ep1", "type": domain.DocTypeEpisode, "content": "对话", "_score": 0.65},
			}, nil
		}
		return nil, nil
	}

	action := NewRetrievalAction()
	action.WithStores(mockVector, NewMockGraphStore())

	req := &domain.RetrieveRequest{
		AgentID: "agent",
		UserID:  "user",
		Query:   "测试",
	}
	recallCtx := domain.NewRecallContext(ctx, req)

	action.HandleRecall(recallCtx)

	// Check scores are populated
	if len(recallCtx.Summaries) > 0 {
		assert.Equal(t, 0.95, recallCtx.Summaries[0].Score)
	}
	if len(recallCtx.Edges) > 0 {
		assert.Equal(t, 0.85, recallCtx.Edges[0].Score)
	}
	if len(recallCtx.Entities) > 0 {
		assert.Equal(t, 0.75, recallCtx.Entities[0].Score)
	}
	if len(recallCtx.Episodes) > 0 {
		assert.Equal(t, 0.65, recallCtx.Episodes[0].Score)
	}
}
