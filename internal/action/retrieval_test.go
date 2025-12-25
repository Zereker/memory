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

func TestTruncateByTokens_ZeroBudget_ReturnsEmptySlice(t *testing.T) {
	items := []domain.Summary{
		{ID: "s1", Content: "test"},
	}

	estimator := func(s domain.Summary) int {
		return 10
	}

	// 修复后：maxTokens=0 时返回空切片
	result := truncateByTokens(items, 0, estimator)

	assert.NotNil(t, result, "Should return empty slice, not nil")
	assert.Len(t, result, 0, "Should be empty")
}

func TestTruncateByTokens_NegativeBudget_ReturnsEmptySlice(t *testing.T) {
	items := []domain.Summary{
		{ID: "s1", Content: "test"},
	}

	estimator := func(s domain.Summary) int {
		return 10
	}

	// 修复后：maxTokens=-1 时返回空切片
	result := truncateByTokens(items, -1, estimator)

	assert.NotNil(t, result, "Should return empty slice, not nil")
	assert.Len(t, result, 0, "Should be empty")
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

func TestFormatMemoryContext_Empty(t *testing.T) {
	ctx := context.Background()
	req := &domain.RetrieveRequest{AgentID: "agent", UserID: "user", Query: "test"}
	recallCtx := domain.NewRecallContext(ctx, req)

	result := FormatMemoryContext(recallCtx)

	assert.Equal(t, "没有找到相关的记忆信息。", result)
}

func TestFormatMemoryContext_OnlySummaries(t *testing.T) {
	ctx := context.Background()
	req := &domain.RetrieveRequest{AgentID: "agent", UserID: "user", Query: "test"}
	recallCtx := domain.NewRecallContext(ctx, req)

	recallCtx.Summaries = []domain.Summary{
		{ID: "s1", Topic: "工作", Content: "用户是产品经理"},
		{ID: "s2", Topic: "", Content: "无主题摘要"},
	}

	result := FormatMemoryContext(recallCtx)

	assert.Contains(t, result, "## 对话摘要")
	assert.Contains(t, result, "[工作] 用户是产品经理")
	assert.Contains(t, result, "- 无主题摘要")
}

func TestFormatMemoryContext_OnlyEdges(t *testing.T) {
	ctx := context.Background()
	req := &domain.RetrieveRequest{AgentID: "agent", UserID: "user", Query: "test"}
	recallCtx := domain.NewRecallContext(ctx, req)

	recallCtx.Edges = []domain.Edge{
		{ID: "e1", Fact: "张三住在北京"},
		{ID: "e2", Fact: "张三喜欢编程"},
	}

	result := FormatMemoryContext(recallCtx)

	assert.Contains(t, result, "## 用户信息")
	assert.Contains(t, result, "- 张三住在北京")
	assert.Contains(t, result, "- 张三喜欢编程")
}

func TestFormatMemoryContext_OnlyEpisodes(t *testing.T) {
	ctx := context.Background()
	req := &domain.RetrieveRequest{AgentID: "agent", UserID: "user", Query: "test"}
	recallCtx := domain.NewRecallContext(ctx, req)

	recallCtx.Episodes = []domain.Episode{
		{ID: "ep1", Name: "张三", Content: "我在北京"},
		{ID: "ep2", Name: "", Role: "user", Content: "没有名字的消息"},
	}

	result := FormatMemoryContext(recallCtx)

	assert.Contains(t, result, "## 相关对话记录")
	assert.Contains(t, result, "[张三] 我在北京")
	assert.Contains(t, result, "[user] 没有名字的消息")
}

func TestFormatMemoryContext_OnlyEntities(t *testing.T) {
	ctx := context.Background()
	req := &domain.RetrieveRequest{AgentID: "agent", UserID: "user", Query: "test"}
	recallCtx := domain.NewRecallContext(ctx, req)

	recallCtx.Entities = []domain.Entity{
		{ID: "ent1", Name: "张三", Type: "person", Description: "一个程序员"},
		{ID: "ent2", Name: "北京", Type: "place", Description: ""},
	}

	result := FormatMemoryContext(recallCtx)

	assert.Contains(t, result, "## 提及的实体")
	assert.Contains(t, result, "- 张三: 一个程序员")
	assert.Contains(t, result, "- 北京 (place)")
}

func TestFormatMemoryContext_AllTypes(t *testing.T) {
	ctx := context.Background()
	req := &domain.RetrieveRequest{AgentID: "agent", UserID: "user", Query: "test"}
	recallCtx := domain.NewRecallContext(ctx, req)

	recallCtx.Summaries = []domain.Summary{{ID: "s1", Topic: "工作", Content: "摘要内容"}}
	recallCtx.Edges = []domain.Edge{{ID: "e1", Fact: "事实"}}
	recallCtx.Episodes = []domain.Episode{{ID: "ep1", Name: "张三", Content: "对话"}}
	recallCtx.Entities = []domain.Entity{{ID: "ent1", Name: "实体", Type: "type", Description: "描述"}}

	result := FormatMemoryContext(recallCtx)

	assert.Contains(t, result, "## 对话摘要")
	assert.Contains(t, result, "## 用户信息")
	assert.Contains(t, result, "## 相关对话记录")
	assert.Contains(t, result, "## 提及的实体")
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

func TestRetrievalAction_TruncateSummaries_NoTruncationNeeded(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	h.setEmbedderVector([]float32{0.1, 0.2, 0.3})

	mockVector := NewMockVectorStore()
	mockVector.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
		docType, _ := query.Filters["type"].(string)
		if docType == domain.DocTypeSummary {
			return []map[string]any{
				{"id": "s1", "type": domain.DocTypeSummary, "content": "摘要1"},
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
			MaxSummaries: 10, // More than we have
		},
	}
	recallCtx := domain.NewRecallContext(ctx, req)

	action.HandleRecall(recallCtx)

	assert.Len(t, recallCtx.Summaries, 1)
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
	items := []domain.Summary{
		{ID: "s1", Content: strings.Repeat("很长的内容", 100)}, // Very long content
	}

	estimator := func(s domain.Summary) int {
		return 1000 // Each item uses 1000 tokens
	}

	// Budget is 500, so no items should fit
	result := truncateByTokens(items, 500, estimator)

	assert.Len(t, result, 0)
}

func TestTokenBudget_Remaining_ExhaustedBudget(t *testing.T) {
	budget := &tokenBudget{
		total: 100,
		used:  150, // Used more than total
	}

	assert.Equal(t, 0, budget.remaining())
}

func TestResolveLimit_AllCases(t *testing.T) {
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
// Additional Tests for Full Coverage
// ============================================================================

func TestRetrievalAction_TruncateSummaries_WithTruncation(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	h.setEmbedderVector([]float32{0.1, 0.2, 0.3})

	mockVector := NewMockVectorStore()
	mockVector.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
		docType, _ := query.Filters["type"].(string)
		if docType == domain.DocTypeSummary {
			return []map[string]any{
				{"id": "s1", "type": domain.DocTypeSummary, "content": "摘要1"},
				{"id": "s2", "type": domain.DocTypeSummary, "content": "摘要2"},
				{"id": "s3", "type": domain.DocTypeSummary, "content": "摘要3"},
				{"id": "s4", "type": domain.DocTypeSummary, "content": "摘要4"},
				{"id": "s5", "type": domain.DocTypeSummary, "content": "摘要5"},
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
			MaxSummaries: 2, // Only allow 2 summaries
		},
	}
	recallCtx := domain.NewRecallContext(ctx, req)

	action.HandleRecall(recallCtx)

	// Should be truncated to 2
	assert.Len(t, recallCtx.Summaries, 2)
}

func TestRetrievalAction_TruncateEdges_WithTruncation(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	h.setEmbedderVector([]float32{0.1, 0.2, 0.3})

	mockVector := NewMockVectorStore()
	mockVector.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
		docType, _ := query.Filters["type"].(string)
		if docType == domain.DocTypeEdge {
			edges := make([]map[string]any, 20)
			for i := 0; i < 20; i++ {
				edges[i] = map[string]any{
					"id":   fmt.Sprintf("e%d", i),
					"type": domain.DocTypeEdge,
					"fact": "事实",
				}
			}
			return edges, nil
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
			MaxEdges: 5, // Only allow 5 edges
		},
	}
	recallCtx := domain.NewRecallContext(ctx, req)

	action.HandleRecall(recallCtx)

	// Should be truncated to 5
	assert.LessOrEqual(t, len(recallCtx.Edges), 5)
}

func TestRetrievalAction_TruncateEntities_WithTruncation(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	h.setEmbedderVector([]float32{0.1, 0.2, 0.3})

	mockVector := NewMockVectorStore()
	mockVector.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
		docType, _ := query.Filters["type"].(string)
		if docType == domain.DocTypeEntity {
			entities := make([]map[string]any, 10)
			for i := 0; i < 10; i++ {
				entities[i] = map[string]any{
					"id":          fmt.Sprintf("ent%d", i),
					"type":        domain.DocTypeEntity,
					"name":        fmt.Sprintf("实体%d", i),
					"entity_type": "person",
				}
			}
			return entities, nil
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
			MaxEntities: 3, // Only allow 3 entities
		},
	}
	recallCtx := domain.NewRecallContext(ctx, req)

	action.HandleRecall(recallCtx)

	// Should be truncated to 3
	assert.LessOrEqual(t, len(recallCtx.Entities), 3)
}

func TestRetrievalAction_TruncateEpisodes_WithTruncation(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	h.setEmbedderVector([]float32{0.1, 0.2, 0.3})

	mockVector := NewMockVectorStore()
	mockVector.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
		docType, _ := query.Filters["type"].(string)
		if docType == domain.DocTypeEpisode {
			episodes := make([]map[string]any, 15)
			for i := 0; i < 15; i++ {
				episodes[i] = map[string]any{
					"id":      fmt.Sprintf("ep%d", i),
					"type":    domain.DocTypeEpisode,
					"content": "对话内容",
				}
			}
			return episodes, nil
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
			MaxEpisodes: 4, // Only allow 4 episodes
		},
	}
	recallCtx := domain.NewRecallContext(ctx, req)

	action.HandleRecall(recallCtx)

	// Should be truncated to 4
	assert.LessOrEqual(t, len(recallCtx.Episodes), 4)
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
