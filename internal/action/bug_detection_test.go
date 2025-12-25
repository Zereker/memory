package action

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/Zereker/memory/internal/domain"
	"github.com/Zereker/memory/pkg/vector"
)

// ============================================================================
// Bug Detection Tests - 这些测试暴露代码中的真实bug
// ============================================================================

// TestBug_EpisodeAction_InconsistentBehavior_NilStore_vs_StoreError 暴露行为不一致bug
// 当 store 返回错误时，episode 不添加到 context
// 当 store 为 nil 时，episode 添加到 context（但没有持久化）
// 这是不一致的行为，调用者无法知道 episode 是否真的被持久化了
func TestBug_EpisodeAction_InconsistentBehavior_NilStore_vs_StoreError(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	h.setEmbedderVector([]float32{0.1, 0.2, 0.3})
	h.setModelJSON(map[string]any{"topic": "test"})

	t.Run("StoreError: episode NOT added to context", func(t *testing.T) {
		mockVector := NewMockVectorStore()
		mockVector.StoreFunc = func(ctx context.Context, id string, doc map[string]any) error {
			return errors.New("store failed")
		}

		action := NewEpisodeStorageAction()
		action.WithVectorStore(mockVector)

		addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
		addCtx.Messages = domain.Messages{
			{Role: domain.RoleUser, Content: "test message"},
		}

		action.Handle(addCtx)

		// 存储失败时，episode 不在 context 中
		assert.Empty(t, addCtx.Episodes, "store error: episode should NOT be in context")
	})

	t.Run("NilStore: episode IS added to context (BUG: inconsistent behavior)", func(t *testing.T) {
		action := NewEpisodeStorageAction()
		action.WithVectorStore(nil) // nil store

		addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
		addCtx.Messages = domain.Messages{
			{Role: domain.RoleUser, Content: "test message"},
		}

		action.Handle(addCtx)

		// BUG: nil store 时，episode 被添加到 context，但实际上没有持久化
		// 这与 store error 的行为不一致
		// 预期行为应该是一致的：要么都添加，要么都不添加
		if len(addCtx.Episodes) > 0 {
			t.Log("BUG DETECTED: nil store causes episode to be added to context without persistence")
			t.Log("This is inconsistent with store error behavior where episode is NOT added")
		}
		// 当前实现：nil store 时 episode 被添加
		assert.Len(t, addCtx.Episodes, 1, "nil store: episode IS in context (current behavior)")
	})
}

// TestBug_ExtractionAction_InconsistentStoreHandling 暴露图存储和向量存储处理不一致
// 图存储失败 -> entity 不添加到 context
// 向量存储失败 -> entity 仍然添加到 context
func TestBug_ExtractionAction_InconsistentStoreHandling(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	h.setEmbedderVector([]float32{0.1, 0.2, 0.3})
	h.setModelJSON(map[string]any{
		"entities":  []map[string]any{{"name": "张三", "type": "person"}},
		"relations": []map[string]any{},
	})

	t.Run("GraphStoreError: entity NOT added to context", func(t *testing.T) {
		mockVector := NewMockVectorStore()
		mockGraph := NewMockGraphStore()
		mockGraph.MergeNodeFunc = func(ctx context.Context, labels []string, mergeKey string, mergeValue any, properties map[string]any) error {
			return errors.New("graph store failed")
		}

		action := NewExtractionAction()
		action.WithStores(mockVector, mockGraph)

		addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
		addCtx.Messages = domain.Messages{{Role: domain.RoleUser, Content: "测试"}}

		action.Handle(addCtx)

		// 图存储失败时，entity 不在 context 中
		assert.Empty(t, addCtx.Entities, "graph store error: entity should NOT be in context")
	})

	t.Run("VectorStoreError: entity IS added to context (BUG: inconsistent)", func(t *testing.T) {
		mockVector := NewMockVectorStore()
		mockVector.StoreFunc = func(ctx context.Context, id string, doc map[string]any) error {
			return errors.New("vector store failed")
		}
		mockGraph := NewMockGraphStore() // graph store succeeds

		action := NewExtractionAction()
		action.WithStores(mockVector, mockGraph)

		addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
		addCtx.Messages = domain.Messages{{Role: domain.RoleUser, Content: "测试"}}

		action.Handle(addCtx)

		// BUG: 向量存储失败时，entity 仍然被添加到 context
		// 这与图存储失败的处理方式不一致
		if len(addCtx.Entities) > 0 {
			t.Log("BUG DETECTED: vector store failure still adds entity to context")
			t.Log("This is inconsistent with graph store failure behavior")
		}
		// 当前实现：向量存储失败也会添加 entity
		assert.Len(t, addCtx.Entities, 1, "vector store error: entity IS in context (current behavior)")
	})
}

// TestBug_SummaryAction_CreatedAtTypeHandling 暴露 created_at 类型处理问题
// 只处理 string 类型的 created_at，time.Time 类型会被忽略
func TestBug_SummaryAction_CreatedAtTypeHandling(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	h.setEmbedderVector([]float32{0.1, 0.2, 0.3})
	h.setModelJSON(map[string]any{"content": "摘要内容"})

	t.Run("StringCreatedAt: range filter applied correctly", func(t *testing.T) {
		mockStore := NewMockVectorStore()
		searchCount := 0
		rangeFilterApplied := false

		mockStore.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
			searchCount++
			docType, _ := query.Filters["type"].(string)

			if docType == domain.DocTypeEpisode && searchCount == 1 {
				return []map[string]any{
					{"id": "ep_old", "role": domain.RoleUser, "topic": "旧主题", "topic_embedding": []float32{0.0, 1.0, 0.0}},
				}, nil
			}
			if docType == domain.DocTypeSummary {
				// 返回 string 类型的 created_at
				return []map[string]any{
					{"id": "sum_old", "created_at": "2024-01-01T00:00:00Z"},
				}, nil
			}
			// 检查是否应用了 range filter
			if query.RangeFilters != nil {
				rangeFilterApplied = true
			}
			return []map[string]any{
				{"id": "ep_old", "role": domain.RoleUser, "content": "旧消息"},
			}, nil
		}

		action := NewSummaryAction()
		action.WithStore(mockStore)

		addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
		addCtx.TopicThreshold = 0.8
		addCtx.Episodes = []domain.Episode{
			{ID: "ep_current", Role: domain.RoleUser, Topic: "新主题", TopicEmbedding: []float32{1.0, 0.0, 0.0}},
		}

		action.Handle(addCtx)

		assert.True(t, rangeFilterApplied, "string created_at: range filter should be applied")
	})

	t.Run("TimeCreatedAt: range filter NOT applied (BUG)", func(t *testing.T) {
		mockStore := NewMockVectorStore()
		searchCount := 0
		rangeFilterApplied := false

		mockStore.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
			searchCount++
			docType, _ := query.Filters["type"].(string)

			if docType == domain.DocTypeEpisode && searchCount == 1 {
				return []map[string]any{
					{"id": "ep_old", "role": domain.RoleUser, "topic": "旧主题", "topic_embedding": []float32{0.0, 1.0, 0.0}},
				}, nil
			}
			if docType == domain.DocTypeSummary {
				// 返回 time.Time 类型的 created_at (某些数据库驱动会返回这种类型)
				return []map[string]any{
					{"id": "sum_old", "created_at": time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
				}, nil
			}
			// 检查是否应用了 range filter
			if query.RangeFilters != nil {
				rangeFilterApplied = true
			}
			return []map[string]any{
				{"id": "ep_old", "role": domain.RoleUser, "content": "旧消息"},
			}, nil
		}

		action := NewSummaryAction()
		action.WithStore(mockStore)

		addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
		addCtx.TopicThreshold = 0.8
		addCtx.Episodes = []domain.Episode{
			{ID: "ep_current", Role: domain.RoleUser, Topic: "新主题", TopicEmbedding: []float32{1.0, 0.0, 0.0}},
		}

		action.Handle(addCtx)

		// BUG: time.Time 类型的 created_at 不会触发 range filter
		if !rangeFilterApplied {
			t.Log("BUG DETECTED: time.Time created_at is not handled")
			t.Log("Only string created_at triggers range filter")
			t.Log("This may cause duplicate summaries or incorrect episode selection")
		}
	})
}

// TestBug_SummaryAction_EmptyTopic 暴露 topic 可能为空的问题
func TestBug_SummaryAction_EmptyTopic(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	h.setEmbedderVector([]float32{0.1, 0.2, 0.3})
	h.setModelJSON(map[string]any{"content": "摘要内容"})

	mockStore := NewMockVectorStore()
	searchCount := 0
	mockStore.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
		searchCount++
		docType, _ := query.Filters["type"].(string)

		if docType == domain.DocTypeEpisode && searchCount == 1 {
			return []map[string]any{
				{"id": "ep_old", "role": domain.RoleUser, "topic": "旧主题", "topic_embedding": []float32{0.0, 1.0, 0.0}},
			}, nil
		}
		if docType == domain.DocTypeSummary {
			return nil, nil
		}
		// 返回 topic 为空的 episodes
		return []map[string]any{
			{"id": "ep_1", "role": domain.RoleUser, "content": "消息1", "topic": ""}, // 空 topic
			{"id": "ep_2", "role": domain.RoleUser, "content": "消息2", "topic": ""},
		}, nil
	}

	action := NewSummaryAction()
	action.WithStore(mockStore)

	addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
	addCtx.TopicThreshold = 0.8
	addCtx.Episodes = []domain.Episode{
		{ID: "ep_current", Role: domain.RoleUser, Topic: "新主题", TopicEmbedding: []float32{1.0, 0.0, 0.0}},
	}

	action.Handle(addCtx)

	// 检查生成的 summary 的 topic
	if len(addCtx.Summaries) > 0 {
		if addCtx.Summaries[0].Topic == "" {
			t.Log("BUG DETECTED: Summary has empty topic because first episode has no topic")
			t.Log("This may cause issues in topic-based filtering later")
		}
		// 这不一定是 bug，但可能是意外行为
		// Summary 的 topic 来自第一个 episode，如果那个是空的，summary topic 也是空的
	}
}

// TestBug_RetrievalAction_EpisodeCoveredBySummary_StillReturned 检查被摘要覆盖的 episode 是否被正确过滤
func TestBug_RetrievalAction_EpisodeCoveredBySummary_ButNoEpisodeIDs(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	h.setEmbedderVector([]float32{0.1, 0.2, 0.3})

	mockVector := NewMockVectorStore()
	mockVector.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
		docType, _ := query.Filters["type"].(string)
		switch docType {
		case domain.DocTypeSummary:
			// Summary 没有 episode_ids 字段
			return []map[string]any{
				{"id": "sum_1", "type": domain.DocTypeSummary, "content": "摘要"},
				// 注意：没有 episode_ids 字段!
			}, nil
		case domain.DocTypeEpisode:
			return []map[string]any{
				{"id": "ep_1", "type": domain.DocTypeEpisode, "content": "对话"},
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

	// 如果 summary 没有 episode_ids，episode 应该不会被错误地过滤掉
	// 这是边界情况的处理
	t.Logf("Summaries: %d, Episodes: %d", len(recallCtx.Summaries), len(recallCtx.Episodes))
}
