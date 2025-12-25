package action

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Zereker/memory/internal/domain"
	"github.com/Zereker/memory/pkg/vector"
)

func TestSummaryAction_Name(t *testing.T) {
	ctx := context.Background()
	_ = newTestHelper(ctx)

	action := NewSummaryAction()
	assert.Equal(t, "summary", action.Name())
}

func TestSummaryAction_WithStore(t *testing.T) {
	ctx := context.Background()
	_ = newTestHelper(ctx)

	mockStore := NewMockVectorStore()
	action := NewSummaryAction()
	result := action.WithStore(mockStore)

	assert.Same(t, action, result)
}

func TestSummaryAction_Handle_NoUserEpisodes(t *testing.T) {
	ctx := context.Background()
	_ = newTestHelper(ctx)

	action := NewSummaryAction()
	action.WithStore(nil)

	addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
	addCtx.Episodes = []domain.Episode{
		{ID: "ep_1", Role: domain.RoleAssistant}, // only assistant
	}

	action.Handle(addCtx)

	assert.Empty(t, addCtx.Summaries)
	assert.NoError(t, addCtx.Error())
}

func TestSummaryAction_Handle_WithTopicChange(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	h.setEmbedderVector([]float32{0.1, 0.2, 0.3})
	h.setModelJSON(map[string]any{
		"content": "用户张三住在北京",
	})

	mockStore := NewMockVectorStore()
	searchCount := 0
	mockStore.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
		searchCount++
		docType, _ := query.Filters["type"].(string)
		if docType == domain.DocTypeEpisode && searchCount == 1 {
			// loadLastUserEpisode - return episode with different topic
			return []map[string]any{
				{
					"id":              "ep_old",
					"role":            domain.RoleUser,
					"topic":           "旧主题",
					"topic_embedding": []float32{0.0, 1.0, 0.0},
				},
			}, nil
		}
		if docType == domain.DocTypeSummary {
			return nil, nil
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
		{
			ID:             "ep_current",
			Role:           domain.RoleUser,
			Topic:          "新主题",
			TopicEmbedding: []float32{1.0, 0.0, 0.0},
		},
	}

	action.Handle(addCtx)

	assert.NoError(t, addCtx.Error())
	assert.Len(t, addCtx.Summaries, 1)
}

func TestSummaryAction_LoadEpisodes_DatabaseErrorPropagated(t *testing.T) {
	ctx := context.Background()
	_ = newTestHelper(ctx)

	dbError := errors.New("database connection failed")

	mockStore := NewMockVectorStore()
	mockStore.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
		// 查询 Summary 时返回错误
		return nil, dbError
	}

	action := NewSummaryAction()
	action.WithStore(mockStore)

	addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
	addCtx.Episodes = []domain.Episode{
		{ID: "ep_current", Role: domain.RoleUser, Topic: "test"},
	}

	action.Handle(addCtx)

	// 修复后：数据库错误应该被传播
	assert.Error(t, addCtx.Error(), "Database error should be propagated")
	assert.Contains(t, addCtx.Error().Error(), "database connection failed")
}

// ============================================================================
// Boundary Cases
// ============================================================================

func TestSummaryAction_Handle_NoHistoricalEpisode(t *testing.T) {
	ctx := context.Background()
	_ = newTestHelper(ctx)

	mockStore := NewMockVectorStore()
	mockStore.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
		// 返回空结果，没有历史 episode
		return nil, nil
	}

	action := NewSummaryAction()
	action.WithStore(mockStore)

	addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
	addCtx.Episodes = []domain.Episode{
		{ID: "ep_current", Role: domain.RoleUser, Topic: "新主题"},
	}

	action.Handle(addCtx)

	// 没有历史 episode，应该跳过摘要生成
	assert.NoError(t, addCtx.Error())
	assert.Empty(t, addCtx.Summaries)
}

func TestSummaryAction_Handle_MissingTopicEmbedding(t *testing.T) {
	ctx := context.Background()
	_ = newTestHelper(ctx)

	mockStore := NewMockVectorStore()
	mockStore.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
		return []map[string]any{
			{
				"id":    "ep_old",
				"role":  domain.RoleUser,
				"topic": "旧主题",
				// 没有 topic_embedding
			},
		}, nil
	}

	action := NewSummaryAction()
	action.WithStore(mockStore)

	addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
	addCtx.Episodes = []domain.Episode{
		{
			ID:             "ep_current",
			Role:           domain.RoleUser,
			Topic:          "新主题",
			TopicEmbedding: []float32{1.0, 0.0, 0.0},
		},
	}

	action.Handle(addCtx)

	// 缺少 embedding，应该跳过摘要生成
	assert.NoError(t, addCtx.Error())
	assert.Empty(t, addCtx.Summaries)
}

func TestSummaryAction_Handle_SimilarTopics(t *testing.T) {
	ctx := context.Background()
	_ = newTestHelper(ctx)

	mockStore := NewMockVectorStore()
	mockStore.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
		return []map[string]any{
			{
				"id":              "ep_old",
				"role":            domain.RoleUser,
				"topic":           "工作",
				"topic_embedding": []float32{1.0, 0.0, 0.0}, // 与当前相同
			},
		}, nil
	}

	action := NewSummaryAction()
	action.WithStore(mockStore)

	addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
	addCtx.TopicThreshold = 0.8
	addCtx.Episodes = []domain.Episode{
		{
			ID:             "ep_current",
			Role:           domain.RoleUser,
			Topic:          "工作",
			TopicEmbedding: []float32{1.0, 0.0, 0.0}, // 相同向量
		},
	}

	action.Handle(addCtx)

	// 主题相似，不生成摘要
	assert.NoError(t, addCtx.Error())
	assert.Empty(t, addCtx.Summaries)
}

func TestSummaryAction_Handle_WithExistingSummary(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	h.setEmbedderVector([]float32{0.1, 0.2, 0.3})
	h.setModelJSON(map[string]any{
		"content": "新的摘要内容",
	})

	mockStore := NewMockVectorStore()
	searchCount := 0
	mockStore.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
		searchCount++
		docType, _ := query.Filters["type"].(string)

		if docType == domain.DocTypeEpisode && searchCount == 1 {
			// loadLastUserEpisode
			return []map[string]any{
				{
					"id":              "ep_old",
					"role":            domain.RoleUser,
					"topic":           "旧主题",
					"topic_embedding": []float32{0.0, 1.0, 0.0},
				},
			}, nil
		}
		if docType == domain.DocTypeSummary {
			// 返回已存在的摘要
			return []map[string]any{
				{
					"id":         "sum_old",
					"created_at": "2024-01-01T00:00:00Z",
				},
			}, nil
		}
		// loadEpisodesSinceLastSummary
		return []map[string]any{
			{"id": "ep_after_summary", "role": domain.RoleUser, "content": "摘要后的消息"},
		}, nil
	}

	action := NewSummaryAction()
	action.WithStore(mockStore)

	addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
	addCtx.TopicThreshold = 0.8
	addCtx.Episodes = []domain.Episode{
		{
			ID:             "ep_current",
			Role:           domain.RoleUser,
			Topic:          "新主题",
			TopicEmbedding: []float32{1.0, 0.0, 0.0},
		},
	}

	action.Handle(addCtx)

	assert.NoError(t, addCtx.Error())
	assert.Len(t, addCtx.Summaries, 1)
}

func TestSummaryAction_Handle_NilStore(t *testing.T) {
	ctx := context.Background()
	_ = newTestHelper(ctx)

	action := NewSummaryAction()
	action.WithStore(nil) // nil store

	addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
	addCtx.Episodes = []domain.Episode{
		{
			ID:             "ep_current",
			Role:           domain.RoleUser,
			Topic:          "主题",
			TopicEmbedding: []float32{1.0, 0.0, 0.0},
		},
	}

	action.Handle(addCtx)

	// nil store 时应该正常跳过
	assert.NoError(t, addCtx.Error())
	assert.Empty(t, addCtx.Summaries)
}

func TestSummaryAction_Handle_NoEpisodesToSummarize(t *testing.T) {
	ctx := context.Background()
	_ = newTestHelper(ctx)

	mockStore := NewMockVectorStore()
	searchCount := 0
	mockStore.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
		searchCount++
		docType, _ := query.Filters["type"].(string)

		if docType == domain.DocTypeEpisode && searchCount == 1 {
			return []map[string]any{
				{
					"id":              "ep_old",
					"role":            domain.RoleUser,
					"topic":           "旧主题",
					"topic_embedding": []float32{0.0, 1.0, 0.0},
				},
			}, nil
		}
		if docType == domain.DocTypeSummary {
			return nil, nil
		}
		// loadEpisodesSinceLastSummary 返回空
		return nil, nil
	}

	action := NewSummaryAction()
	action.WithStore(mockStore)

	addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
	addCtx.TopicThreshold = 0.8
	addCtx.Episodes = []domain.Episode{
		{
			ID:             "ep_current",
			Role:           domain.RoleUser,
			Topic:          "新主题",
			TopicEmbedding: []float32{1.0, 0.0, 0.0},
		},
	}

	action.Handle(addCtx)

	// 没有 episodes 可以总结
	assert.NoError(t, addCtx.Error())
	assert.Empty(t, addCtx.Summaries)
}

func TestSummaryAction_Handle_LoadEpisodesSinceLastSummaryError(t *testing.T) {
	ctx := context.Background()
	_ = newTestHelper(ctx)

	mockStore := NewMockVectorStore()
	searchCount := 0
	mockStore.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
		searchCount++
		docType, _ := query.Filters["type"].(string)

		if docType == domain.DocTypeEpisode && searchCount == 1 {
			return []map[string]any{
				{
					"id":              "ep_old",
					"role":            domain.RoleUser,
					"topic":           "旧主题",
					"topic_embedding": []float32{0.0, 1.0, 0.0},
				},
			}, nil
		}
		if docType == domain.DocTypeSummary {
			return nil, errors.New("summary query failed")
		}
		return nil, nil
	}

	action := NewSummaryAction()
	action.WithStore(mockStore)

	addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
	addCtx.TopicThreshold = 0.8
	addCtx.Episodes = []domain.Episode{
		{
			ID:             "ep_current",
			Role:           domain.RoleUser,
			Topic:          "新主题",
			TopicEmbedding: []float32{1.0, 0.0, 0.0},
		},
	}

	action.Handle(addCtx)

	// 错误应该被传播
	assert.Error(t, addCtx.Error())
	assert.Contains(t, addCtx.Error().Error(), "summary query failed")
}
