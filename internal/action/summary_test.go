package action

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Zereker/memory/internal/domain"
	"github.com/Zereker/memory/pkg/vector"
)

func TestSummaryAction_Name(t *testing.T) {
	ctx := context.Background()
	_ = NewTestHelper(ctx)

	action := NewSummaryAction()
	assert.Equal(t, "summary", action.Name())
}

func TestSummaryAction_WithStore(t *testing.T) {
	ctx := context.Background()
	_ = NewTestHelper(ctx)

	mockStore := NewMockVectorStore()
	action := NewSummaryAction()
	result := action.WithStore(mockStore)

	assert.Same(t, action, result)
}

func TestSummaryAction_Handle_NoUserEpisodes(t *testing.T) {
	ctx := context.Background()
	helper := NewTestHelper(ctx)

	action := helper.NewSummaryAction()
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
	helper := NewTestHelper(ctx)
	helper.SetEmbedderVector([]float32{0.1, 0.2, 0.3})
	helper.SetModelJSON(map[string]any{
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

	action := helper.NewSummaryAction()
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
