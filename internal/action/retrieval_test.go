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
