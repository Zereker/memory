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

// ============================================================================
// Additional Coverage Tests for summary.go
// ============================================================================

func TestSummaryAction_Handle_GenerateSummaryError(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	h.setEmbedderVector([]float32{0.1, 0.2, 0.3})
	h.MockPlugin.SetModelResponse("doubao-pro-32k", func(ctx context.Context, req *ai.ModelRequest) (*ai.ModelResponse, error) {
		return nil, errors.New("summary generation failed")
	})

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
		// loadEpisodesSinceLastSummary
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

	// LLM error should result in no summary, but no error propagated
	assert.Empty(t, addCtx.Summaries)
	assert.NoError(t, addCtx.Error())
}

func TestSummaryAction_Handle_EmbeddingError(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	h.setModelJSON(map[string]any{
		"content": "生成的摘要内容",
	})
	h.MockPlugin.SetEmbedderResponse("doubao-embedding-text-240715", func(ctx context.Context, req *ai.EmbedRequest) (*ai.EmbedResponse, error) {
		return nil, errors.New("embedding failed")
	})

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
		// loadEpisodesSinceLastSummary
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

	// Summary should still be created even if embedding fails
	assert.Len(t, addCtx.Summaries, 1)
	assert.Empty(t, addCtx.Summaries[0].Embedding)
}

func TestSummaryAction_Handle_StoreSummaryError(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	h.setEmbedderVector([]float32{0.1, 0.2, 0.3})
	h.setModelJSON(map[string]any{
		"content": "生成的摘要内容",
	})

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
		// loadEpisodesSinceLastSummary
		return []map[string]any{
			{"id": "ep_old", "role": domain.RoleUser, "content": "旧消息"},
		}, nil
	}
	mockStore.StoreFunc = func(ctx context.Context, id string, doc map[string]any) error {
		return errors.New("store failed")
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

	// Store error should result in no summary added to context
	assert.Empty(t, addCtx.Summaries)
	assert.NoError(t, addCtx.Error())
}

func TestSummaryAction_Handle_LoadEpisodesSinceLastSummary_EpisodeQueryError(t *testing.T) {
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
			return nil, nil // No summary
		}
		// Episode query after summary check fails
		return nil, errors.New("episode query failed")
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

	// Episode query error should be propagated
	assert.Error(t, addCtx.Error())
	assert.Contains(t, addCtx.Error().Error(), "episode query failed")
}

func TestSummaryAction_Handle_LoadLastUserEpisodeError(t *testing.T) {
	ctx := context.Background()
	_ = newTestHelper(ctx)

	mockStore := NewMockVectorStore()
	mockStore.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
		return nil, errors.New("load last user episode failed")
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

	// Error should be propagated
	assert.Error(t, addCtx.Error())
	assert.Contains(t, addCtx.Error().Error(), "load last user episode failed")
}

func TestSummaryAction_Handle_TopicEmbeddingTypes(t *testing.T) {
	tests := []struct {
		name            string
		topicEmbedding  any
		expectSummaries int
		setupModel      bool
	}{
		{
			name:            "float32 slice generates summary",
			topicEmbedding:  []float32{0.0, 1.0, 0.0},
			expectSummaries: 1,
			setupModel:      true,
		},
		{
			name:            "any slice generates summary",
			topicEmbedding:  []any{0.0, 1.0, 0.0},
			expectSummaries: 1,
			setupModel:      true,
		},
		{
			name:            "unknown type skips summary",
			topicEmbedding:  "not a slice",
			expectSummaries: 0,
			setupModel:      false,
		},
		{
			name:            "nil skips summary",
			topicEmbedding:  nil,
			expectSummaries: 0,
			setupModel:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			h := newTestHelper(ctx)
			if tt.setupModel {
				h.setEmbedderVector([]float32{0.1, 0.2, 0.3})
				h.setModelJSON(map[string]any{"content": "生成的摘要内容"})
			}

			mockStore := NewMockVectorStore()
			searchCount := 0
			mockStore.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
				searchCount++
				docType, _ := query.Filters["type"].(string)

				if docType == domain.DocTypeEpisode && searchCount == 1 {
					doc := map[string]any{
						"id":    "ep_old",
						"role":  domain.RoleUser,
						"topic": "旧主题",
					}
					if tt.topicEmbedding != nil {
						doc["topic_embedding"] = tt.topicEmbedding
					}
					return []map[string]any{doc}, nil
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
			assert.Len(t, addCtx.Summaries, tt.expectSummaries)
		})
	}
}

func TestSummaryAction_formatEpisodes(t *testing.T) {
	action := NewSummaryAction()

	episodes := []domain.Episode{
		{Name: "张三", Content: "你好"},
		{Name: "", Role: domain.RoleAssistant, Content: "你好呀"},
		{Name: "李四", Content: "再见"},
	}

	result := action.formatEpisodes(episodes)

	assert.Contains(t, result, "张三: 你好")
	assert.Contains(t, result, "assistant: 你好呀") // Falls back to Role when Name is empty
	assert.Contains(t, result, "李四: 再见")
}

func TestSummaryAction_Handle_ExistingSummaryWithCreatedAt(t *testing.T) {
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
			return []map[string]any{
				{
					"id":         "sum_old",
					"created_at": "2024-01-01T00:00:00Z",
				},
			}, nil
		}
		// Episode query should have range filter now
		if query.RangeFilters != nil {
			return []map[string]any{
				{"id": "ep_after", "role": domain.RoleUser, "content": "摘要后的消息"},
			}, nil
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

	assert.NoError(t, addCtx.Error())
	assert.Len(t, addCtx.Summaries, 1)
}

func TestSummaryAction_Handle_SkipCurrentEpisodeInHistoricalQuery(t *testing.T) {
	ctx := context.Background()
	_ = newTestHelper(ctx)

	mockStore := NewMockVectorStore()
	mockStore.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
		// Return the current episode itself (should be skipped)
		return []map[string]any{
			{
				"id":              "ep_current", // Same as current
				"role":            domain.RoleUser,
				"topic":           "主题",
				"topic_embedding": []float32{0.0, 1.0, 0.0},
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
			Topic:          "主题",
			TopicEmbedding: []float32{1.0, 0.0, 0.0},
		},
	}

	action.Handle(addCtx)

	// Should skip because only found current episode
	assert.NoError(t, addCtx.Error())
	assert.Empty(t, addCtx.Summaries)
}
