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

func TestSummaryAction_Basic(t *testing.T) {
	t.Run("Name returns correct name", func(t *testing.T) {
		ctx := context.Background()
		_ = newTestHelper(ctx)

		action := NewSummaryAction()
		assert.Equal(t, "summary", action.Name())
	})

	t.Run("WithStore returns same instance for chaining", func(t *testing.T) {
		ctx := context.Background()
		_ = newTestHelper(ctx)

		mockStore := NewMockVectorStore()
		action := NewSummaryAction()
		result := action.WithStore(mockStore)

		assert.Same(t, action, result, "should return same instance for chaining")
	})

	t.Run("Handle with no user episodes produces no summaries", func(t *testing.T) {
		ctx := context.Background()
		_ = newTestHelper(ctx)

		action := NewSummaryAction()
		action.WithStore(nil)

		addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
		addCtx.Episodes = []domain.Episode{
			{ID: "ep_1", Role: domain.RoleAssistant},
		}

		action.Handle(addCtx)

		assert.Empty(t, addCtx.Summaries, "assistant-only episodes should produce no summaries")
		assert.NoError(t, addCtx.Error())
	})
}

func TestSummaryAction_Handle_Success(t *testing.T) {
	t.Run("WithTopicChange generates summary", func(t *testing.T) {
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
		assert.Len(t, addCtx.Summaries, 1, "topic change should trigger summary generation")
	})

	t.Run("WithExistingSummary generates new summary from recent episodes", func(t *testing.T) {
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
	})
}

func TestSummaryAction_Handle_SkipConditions(t *testing.T) {
	t.Run("NoHistoricalEpisode skips summary", func(t *testing.T) {
		ctx := context.Background()
		_ = newTestHelper(ctx)

		mockStore := NewMockVectorStore()
		mockStore.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
			return nil, nil
		}

		action := NewSummaryAction()
		action.WithStore(mockStore)

		addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
		addCtx.Episodes = []domain.Episode{
			{ID: "ep_current", Role: domain.RoleUser, Topic: "新主题"},
		}

		action.Handle(addCtx)

		assert.NoError(t, addCtx.Error())
		assert.Empty(t, addCtx.Summaries, "no historical episode should skip summary")
	})

	t.Run("MissingTopicEmbedding skips summary", func(t *testing.T) {
		ctx := context.Background()
		_ = newTestHelper(ctx)

		mockStore := NewMockVectorStore()
		mockStore.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
			return []map[string]any{
				{
					"id":    "ep_old",
					"role":  domain.RoleUser,
					"topic": "旧主题",
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

		assert.NoError(t, addCtx.Error())
		assert.Empty(t, addCtx.Summaries, "missing embedding should skip summary")
	})

	t.Run("SimilarTopics skips summary", func(t *testing.T) {
		ctx := context.Background()
		_ = newTestHelper(ctx)

		mockStore := NewMockVectorStore()
		mockStore.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
			return []map[string]any{
				{
					"id":              "ep_old",
					"role":            domain.RoleUser,
					"topic":           "工作",
					"topic_embedding": []float32{1.0, 0.0, 0.0},
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
				TopicEmbedding: []float32{1.0, 0.0, 0.0},
			},
		}

		action.Handle(addCtx)

		assert.NoError(t, addCtx.Error())
		assert.Empty(t, addCtx.Summaries, "similar topics (cosine similarity >= threshold) should skip summary")
	})

	t.Run("NilStore skips summary gracefully", func(t *testing.T) {
		ctx := context.Background()
		_ = newTestHelper(ctx)

		action := NewSummaryAction()
		action.WithStore(nil)

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

		assert.NoError(t, addCtx.Error())
		assert.Empty(t, addCtx.Summaries)
	})

	t.Run("NoEpisodesToSummarize skips summary", func(t *testing.T) {
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
		assert.Empty(t, addCtx.Summaries, "empty episode list should skip summary")
	})

	t.Run("SkipCurrentEpisodeInHistoricalQuery", func(t *testing.T) {
		ctx := context.Background()
		_ = newTestHelper(ctx)

		mockStore := NewMockVectorStore()
		mockStore.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
			return []map[string]any{
				{
					"id":              "ep_current",
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

		assert.NoError(t, addCtx.Error())
		assert.Empty(t, addCtx.Summaries, "should skip when only found current episode")
	})
}

func TestSummaryAction_Handle_Errors(t *testing.T) {
	t.Run("DatabaseError propagates correctly", func(t *testing.T) {
		ctx := context.Background()
		_ = newTestHelper(ctx)

		mockStore := NewMockVectorStore()
		mockStore.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
			return nil, errors.New("database connection failed")
		}

		action := NewSummaryAction()
		action.WithStore(mockStore)

		addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
		addCtx.Episodes = []domain.Episode{
			{ID: "ep_current", Role: domain.RoleUser, Topic: "test"},
		}

		action.Handle(addCtx)

		assert.Error(t, addCtx.Error(), "database error should be propagated")
		assert.Contains(t, addCtx.Error().Error(), "database connection failed")
	})

	t.Run("SummaryQueryError propagates correctly", func(t *testing.T) {
		ctx := context.Background()
		_ = newTestHelper(ctx)

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
				return nil, errors.New("summary query failed")
			}
			return nil, nil
		}

		action := NewSummaryAction()
		action.WithStore(mockStore)

		addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
		addCtx.TopicThreshold = 0.8
		addCtx.Episodes = []domain.Episode{
			{ID: "ep_current", Role: domain.RoleUser, Topic: "新主题", TopicEmbedding: []float32{1.0, 0.0, 0.0}},
		}

		action.Handle(addCtx)

		assert.Error(t, addCtx.Error())
		assert.Contains(t, addCtx.Error().Error(), "summary query failed")
	})

	t.Run("EpisodeQueryError propagates correctly", func(t *testing.T) {
		ctx := context.Background()
		_ = newTestHelper(ctx)

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
			return nil, errors.New("episode query failed")
		}

		action := NewSummaryAction()
		action.WithStore(mockStore)

		addCtx := domain.NewAddContext(ctx, "agent", "user", "session")
		addCtx.TopicThreshold = 0.8
		addCtx.Episodes = []domain.Episode{
			{ID: "ep_current", Role: domain.RoleUser, Topic: "新主题", TopicEmbedding: []float32{1.0, 0.0, 0.0}},
		}

		action.Handle(addCtx)

		assert.Error(t, addCtx.Error())
		assert.Contains(t, addCtx.Error().Error(), "episode query failed")
	})

	t.Run("LLMGenerationError skips summary without aborting", func(t *testing.T) {
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
					{"id": "ep_old", "role": domain.RoleUser, "topic": "旧主题", "topic_embedding": []float32{0.0, 1.0, 0.0}},
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
			{ID: "ep_current", Role: domain.RoleUser, Topic: "新主题", TopicEmbedding: []float32{1.0, 0.0, 0.0}},
		}

		action.Handle(addCtx)

		assert.Empty(t, addCtx.Summaries, "LLM error should result in no summary")
		assert.NoError(t, addCtx.Error(), "LLM error should not abort chain")
	})

	t.Run("EmbeddingError creates summary without embedding", func(t *testing.T) {
		ctx := context.Background()
		h := newTestHelper(ctx)
		h.setModelJSON(map[string]any{"content": "生成的摘要内容"})
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
					{"id": "ep_old", "role": domain.RoleUser, "topic": "旧主题", "topic_embedding": []float32{0.0, 1.0, 0.0}},
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
			{ID: "ep_current", Role: domain.RoleUser, Topic: "新主题", TopicEmbedding: []float32{1.0, 0.0, 0.0}},
		}

		action.Handle(addCtx)

		assert.Len(t, addCtx.Summaries, 1, "summary should be created despite embedding failure")
		assert.Empty(t, addCtx.Summaries[0].Embedding, "embedding should be empty on failure")
	})

	t.Run("StoreSummaryError skips summary without aborting", func(t *testing.T) {
		ctx := context.Background()
		h := newTestHelper(ctx)
		h.setEmbedderVector([]float32{0.1, 0.2, 0.3})
		h.setModelJSON(map[string]any{"content": "生成的摘要内容"})

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
			{ID: "ep_current", Role: domain.RoleUser, Topic: "新主题", TopicEmbedding: []float32{1.0, 0.0, 0.0}},
		}

		action.Handle(addCtx)

		assert.Empty(t, addCtx.Summaries, "store error should skip summary")
		assert.NoError(t, addCtx.Error(), "store error should not abort chain")
	})
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

func TestSummaryAction_Handle_ExistingSummaryWithRangeFilter(t *testing.T) {
	ctx := context.Background()
	h := newTestHelper(ctx)
	h.setEmbedderVector([]float32{0.1, 0.2, 0.3})
	h.setModelJSON(map[string]any{"content": "新的摘要内容"})

	mockStore := NewMockVectorStore()
	searchCount := 0
	rangeFilterUsed := false
	mockStore.SearchFunc = func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
		searchCount++
		docType, _ := query.Filters["type"].(string)

		if docType == domain.DocTypeEpisode && searchCount == 1 {
			return []map[string]any{
				{"id": "ep_old", "role": domain.RoleUser, "topic": "旧主题", "topic_embedding": []float32{0.0, 1.0, 0.0}},
			}, nil
		}
		if docType == domain.DocTypeSummary {
			return []map[string]any{
				{"id": "sum_old", "created_at": "2024-01-01T00:00:00Z"},
			}, nil
		}
		if query.RangeFilters != nil {
			rangeFilterUsed = true
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
		{ID: "ep_current", Role: domain.RoleUser, Topic: "新主题", TopicEmbedding: []float32{1.0, 0.0, 0.0}},
	}

	action.Handle(addCtx)

	assert.NoError(t, addCtx.Error())
	assert.Len(t, addCtx.Summaries, 1)
	assert.True(t, rangeFilterUsed, "should use range filter when existing summary has created_at")
}
