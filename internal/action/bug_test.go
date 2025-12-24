package action

import (
	"context"
	"errors"
	"testing"

	"github.com/firebase/genkit/go/ai"
	"github.com/stretchr/testify/assert"

	"github.com/Zereker/memory/internal/domain"
	"github.com/Zereker/memory/pkg/vector"
)

// ============================================================================
// Bug #1: summary.go:176 - 数据库错误被忽略
// summaries, _ := a.store.Search(...) 忽略了错误
// ============================================================================

func TestSummaryAction_LoadEpisodes_DatabaseErrorIgnored(t *testing.T) {
	ctx := context.Background()
	helper := NewTestHelper(ctx)

	dbError := errors.New("database connection failed")

	mockStore := NewMockVectorStore()
	searchCount := 0
	mockStore.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
		searchCount++
		if searchCount == 1 {
			// 第一次查询 Summary 时返回错误
			// BUG: 这个错误被 _ 忽略了
			return nil, dbError
		}
		// 第二次查询 Episode
		return []map[string]any{
			{"id": "ep_1", "role": domain.RoleUser, "content": "test"},
		}, nil
	}

	action := helper.NewSummaryAction()
	action.WithStore(mockStore)

	addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
	addCtx.Episodes = []domain.Episode{
		{ID: "ep_current", Role: domain.RoleUser, Topic: "test"},
	}

	// 期望：数据库错误应该被传播，而不是静默忽略
	// 实际：代码忽略了错误，继续执行
	action.Handle(addCtx)

	// 这个测试暴露了问题：即使数据库出错，代码也没有报错
	// 如果这是预期行为，应该有文档说明；如果不是，应该修复
	t.Log("BUG: Database error on summary search was silently ignored")
	t.Log("Search was called", searchCount, "times")

	// 验证错误被忽略了 - 代码继续执行而没有设置错误
	assert.NoError(t, addCtx.Error(), "Error was silently ignored - this may be a bug")
}

// ============================================================================
// Bug #2: retrieval.go:512 - truncateByTokens 返回 nil 而不是空切片
// ============================================================================

func TestTruncateByTokens_ZeroBudget_ReturnsNilInsteadOfEmptySlice(t *testing.T) {
	items := []domain.Summary{
		{ID: "s1", Content: "test"},
	}

	estimator := func(s domain.Summary) int {
		return 10
	}

	// BUG: maxTokens=0 时返回 nil 而不是空切片
	result := truncateByTokens(items, 0, estimator)

	// 这暴露了 bug：返回 nil 而不是 []domain.Summary{}
	// nil 和空切片在某些场景下行为不同
	assert.Nil(t, result, "Currently returns nil - should this be empty slice instead?")

	// 如果调用方做 append，nil 和空切片行为相同
	// 但如果调用方做 JSON 序列化，nil 变成 null，空切片变成 []
	// 这可能导致 API 返回不一致

	t.Log("BUG: truncateByTokens returns nil instead of empty slice when maxTokens=0")
	t.Log("This could cause JSON serialization issues (null vs [])")
}

func TestTruncateByTokens_NegativeBudget_ReturnsNilInsteadOfEmptySlice(t *testing.T) {
	items := []domain.Summary{
		{ID: "s1", Content: "test"},
	}

	estimator := func(s domain.Summary) int {
		return 10
	}

	// BUG: maxTokens=-1 时返回 nil
	result := truncateByTokens(items, -1, estimator)

	assert.Nil(t, result, "Currently returns nil for negative budget")
	t.Log("BUG: truncateByTokens returns nil for negative maxTokens")
}

// ============================================================================
// Bug #3: 类型断言没有检查 ok
// ============================================================================

func TestRetrievalAction_ScoreTypeAssertion_PanicsOnWrongType(t *testing.T) {
	ctx := context.Background()
	helper := NewTestHelper(ctx)
	helper.SetEmbedderVector([]float32{0.1, 0.2, 0.3})

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
					// BUG: _score 是 string 而不是 float64
					// 代码中 doc["_score"].(float64) 没有检查 ok
					"_score": "0.95", // 故意用错误类型
				},
			}, nil
		}
		return nil, nil
	}

	action := helper.NewRetrievalAction()
	action.WithStores(mockVector, NewMockGraphStore())

	req := &domain.RetrieveRequest{
		AgentID: "agent",
		UserID:  "user",
		Query:   "test",
	}
	recallCtx := domain.NewRecallContext(ctx, req)

	// 这不会 panic，因为类型断言失败时返回零值
	// 但这意味着 score 会静默变成 0，可能影响排序
	action.HandleRecall(recallCtx)

	if len(recallCtx.Episodes) > 0 {
		t.Log("BUG: Score type assertion failed silently, score is now:", recallCtx.Episodes[0].Score)
		// Score 应该是 0.95，但因为类型错误变成了 0
		assert.Equal(t, float64(0), recallCtx.Episodes[0].Score, "Score became 0 due to failed type assertion")
	}
}

// ============================================================================
// Bug #4: embedding 生成失败时静默跳过
// ============================================================================

func TestEpisodeAction_EmbeddingFailure_SilentlySkipped(t *testing.T) {
	ctx := context.Background()
	helper := NewTestHelper(ctx)

	// 设置 embedder 返回错误
	helper.MockPlugin.SetEmbedderResponse("doubao-embedding-text-240715", func(ctx context.Context, req *ai.EmbedRequest) (*ai.EmbedResponse, error) {
		return nil, errors.New("embedding service unavailable")
	})
	helper.SetModelJSON(map[string]any{"topic": "test"})

	mockStore := NewMockVectorStore()
	action := helper.NewEpisodeStorageAction()
	action.WithVectorStore(mockStore)

	addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
	addCtx.Messages = domain.Messages{
		{Role: domain.RoleUser, Content: "test message"},
	}

	action.Handle(addCtx)

	// BUG: embedding 生成失败，但 episode 仍然被创建（只是没有 embedding）
	// 这可能导致后续向量搜索找不到这些 episode
	if len(addCtx.Episodes) > 0 {
		t.Log("Episode was created despite embedding failure")
		t.Log("Embedding length:", len(addCtx.Episodes[0].Embedding))
		// embedding 应该为空或 nil
		assert.Empty(t, addCtx.Episodes[0].Embedding, "Episode has no embedding due to failure")
	}

	// 没有错误被设置 - 这是 bug 还是预期行为？
	t.Log("BUG: Embedding failure was silently ignored, no error set")
}

// ============================================================================
// Bug #5: store 为 nil 时返回 nil 而不是 error
// ============================================================================

func TestEpisodeAction_NilStore_SilentlySucceeds(t *testing.T) {
	ctx := context.Background()
	helper := NewTestHelper(ctx)
	helper.SetEmbedderVector([]float32{0.1, 0.2, 0.3})
	helper.SetModelJSON(map[string]any{"topic": "test"})

	action := helper.NewEpisodeStorageAction()
	// 不设置 store，保持 nil
	action.WithVectorStore(nil)

	addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
	addCtx.Messages = domain.Messages{
		{Role: domain.RoleUser, Content: "test message"},
	}

	action.Handle(addCtx)

	// BUG: store 为 nil 时，storeEpisode 返回 nil 而不是 error
	// 调用方无法知道数据是否真正被存储
	assert.NoError(t, addCtx.Error(), "No error even though store is nil")
	assert.Len(t, addCtx.Episodes, 1, "Episode was 'created' but not actually stored")

	t.Log("BUG: Episode appears to be created successfully, but was never stored (nil store)")
	t.Log("This could lead to data loss without any indication")
}

// ============================================================================
// Bug #6: DocToEpisode 解析失败返回空 struct
// ============================================================================

func TestDocToEpisode_InvalidData_ReturnsEmptyStructInsteadOfError(t *testing.T) {
	action := NewBaseAction("test")

	// 传入无效数据
	doc := map[string]any{
		"id":         123,     // 应该是 string
		"created_at": "invalid-date-format",
		"content_embedding": "not-a-slice", // 应该是 []float32
	}

	ep := action.DocToEpisode(doc)

	// BUG: 解析失败返回空 struct，调用方无法区分：
	// 1. 数据确实为空
	// 2. 解析失败
	assert.NotNil(t, ep, "Returns empty struct instead of nil")
	assert.Empty(t, ep.ID, "ID parsing failed silently")
	assert.Nil(t, ep.Embedding, "Embedding parsing failed silently")

	t.Log("BUG: DocToEpisode returns empty struct on parse failure")
	t.Log("Caller cannot distinguish between empty data and parse error")
}
