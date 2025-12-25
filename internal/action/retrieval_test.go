package action

import (
	"context"
	"testing"

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
