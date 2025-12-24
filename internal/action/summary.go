package action

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Zereker/memory/internal/domain"
	"github.com/Zereker/memory/pkg/storage"
)

// 确保实现 domain.AddAction 接口
var _ domain.AddAction = (*SummaryAction)(nil)

// SummaryAction 摘要生成 Action
// 检测主题变化并生成摘要
type SummaryAction struct {
	*BaseAction
	store *storage.OpenSearchStore
}

// NewSummaryAction 创建 SummaryAction
func NewSummaryAction() *SummaryAction {
	return &SummaryAction{
		BaseAction: NewBaseAction("summary"),
		store:      storage.NewStore(),
	}
}

// Name 返回 action 名称
func (a *SummaryAction) Name() string {
	return "summary"
}

// Handle 执行摘要生成
func (a *SummaryAction) Handle(c *domain.AddContext) {
	a.logger.Info("executing", "episodes", len(c.Episodes))

	// 获取本次请求的 user Episode
	var currentUserEpisode *domain.Episode
	for i := range c.Episodes {
		if c.Episodes[i].Role == domain.RoleUser {
			currentUserEpisode = &c.Episodes[i]
			break
		}
	}

	if currentUserEpisode == nil {
		a.logger.Debug("no user episode in current request, skipping")
		c.Next()
		return
	}

	// 加载历史最近的 user Episode
	lastUserEpisode, err := a.loadLastUserEpisode(c, currentUserEpisode.ID)
	if err != nil {
		a.logger.Warn("failed to load last user episode", "error", err)
		c.Next()
		return
	}

	if lastUserEpisode == nil {
		a.logger.Info("no historical user episode, skipping")
		c.Next()
		return
	}

	// 计算 topic embedding 相似度
	if len(lastUserEpisode.TopicEmbedding) == 0 || len(currentUserEpisode.TopicEmbedding) == 0 {
		a.logger.Info("missing topic embedding, skipping",
			"last_emb_len", len(lastUserEpisode.TopicEmbedding),
			"current_emb_len", len(currentUserEpisode.TopicEmbedding),
		)
		c.Next()
		return
	}

	similarity := a.CosineSimilarity(lastUserEpisode.TopicEmbedding, currentUserEpisode.TopicEmbedding)
	a.logger.Info("topic similarity",
		"last_topic", lastUserEpisode.Topic,
		"current_topic", currentUserEpisode.Topic,
		"similarity", similarity,
		"threshold", c.TopicThreshold,
	)

	// 主题相似，无需生成摘要
	if similarity >= c.TopicThreshold {
		c.Next()
		return
	}

	// 主题变化：生成摘要
	a.logger.Info("topic change detected",
		"similarity", similarity,
		"threshold", c.TopicThreshold,
	)

	// 加载需要生成摘要的历史 Episodes
	episodes, err := a.loadEpisodesSinceLastSummary(c, currentUserEpisode.ID)
	if err != nil {
		a.logger.Warn("failed to load history episodes", "error", err)
		c.Next()
		return
	}

	if len(episodes) == 0 {
		a.logger.Info("no episodes to summarize")
		c.Next()
		return
	}

	a.generateAndStoreSummary(c, episodes)
	c.Next()
}

// loadLastUserEpisode 加载该 session 最近的 user Episode（排除当前）
func (a *SummaryAction) loadLastUserEpisode(c *domain.AddContext, excludeID string) (*domain.Episode, error) {
	if a.store == nil {
		return nil, nil
	}

	results, err := a.store.Search(c.Context, storage.SearchQuery{
		Filters: map[string]any{
			"type":       domain.DocTypeEpisode,
			"agent_id":   c.AgentID,
			"user_id":    c.UserID,
			"session_id": c.SessionID,
			"role":       domain.RoleUser,
		},
		Limit: 2,
	})
	if err != nil {
		return nil, err
	}

	for _, doc := range results {
		id, _ := doc["id"].(string)
		if id != excludeID {
			// Debug: 检查 topic_embedding
			if te, ok := doc["topic_embedding"]; ok {
				switch v := te.(type) {
				case []float32:
					a.logger.Info("doc topic_embedding", "id", id, "type", "[]float32", "len", len(v))
				case []any:
					a.logger.Info("doc topic_embedding", "id", id, "type", "[]any", "len", len(v))
				default:
					a.logger.Info("doc topic_embedding", "id", id, "type", "unknown")
				}
			} else {
				a.logger.Info("doc topic_embedding not found", "id", id)
			}
			ep := a.DocToEpisode(doc)
			a.logger.Info("DocToEpisode result", "id", ep.ID, "topic", ep.Topic, "topic_emb_len", len(ep.TopicEmbedding))
			return ep, nil
		}
	}

	return nil, nil
}

// loadEpisodesSinceLastSummary 加载从上次摘要到本次变化之前的 Episodes
func (a *SummaryAction) loadEpisodesSinceLastSummary(c *domain.AddContext, excludeID string) ([]domain.Episode, error) {
	if a.store == nil {
		return nil, nil
	}

	// 1. 查询该用户最近的 Summary
	summaries, _ := a.store.Search(c.Context, storage.SearchQuery{
		Filters: map[string]any{
			"type":     domain.DocTypeSummary,
			"agent_id": c.AgentID,
			"user_id":  c.UserID,
		},
		Limit: 1,
	})

	// 2. 构建 Episode 查询
	episodeQuery := storage.SearchQuery{
		Filters: map[string]any{
			"type":       domain.DocTypeEpisode,
			"agent_id":   c.AgentID,
			"user_id":    c.UserID,
			"session_id": c.SessionID,
		},
		Limit: 1000,
	}

	// 如果有上次 Summary，只查询其后的 Episodes
	if len(summaries) > 0 {
		if createdAt, ok := summaries[0]["created_at"].(string); ok {
			a.logger.Debug("filtering episodes after summary", "summary_created_at", createdAt)
			episodeQuery.RangeFilters = map[string]map[string]any{
				"created_at": {"gt": createdAt},
			}
		}
	}

	// 3. 查询 Episodes
	results, err := a.store.Search(c.Context, episodeQuery)
	if err != nil {
		return nil, err
	}

	var episodes []domain.Episode
	for _, doc := range results {
		if id, ok := doc["id"].(string); ok && id != excludeID {
			episodes = append(episodes, *a.DocToEpisode(doc))
		}
	}

	return episodes, nil
}

// SummaryResult summary prompt 输出
type SummaryResult struct {
	Content string `json:"content"`
}

// generateAndStoreSummary 生成并存储摘要
func (a *SummaryAction) generateAndStoreSummary(c *domain.AddContext, episodes []domain.Episode) {
	if len(episodes) == 0 {
		return
	}

	a.logger.Info("generating summary", "episode_count", len(episodes))

	// 构建对话文本
	conversation := a.formatEpisodes(episodes)

	// 调用 LLM 生成摘要
	var summaryResult SummaryResult
	if err := a.Generate(c, "summary", map[string]any{
		"conversation": conversation,
		"language":     c.LanguageName(),
	}, &summaryResult); err != nil {
		a.logger.Warn("failed to generate summary", "error", err)
		return
	}

	// 生成 embedding
	embedding, err := a.GenEmbedding(c.Context, EmbedderName, summaryResult.Content)
	if err != nil {
		a.logger.Warn("failed to generate embedding", "error", err)
	}

	// 收集 Episode IDs
	episodeIDs := make([]string, len(episodes))
	for i, ep := range episodes {
		episodeIDs[i] = ep.ID
	}

	// 使用第一个 Episode 的 topic 作为 Summary topic
	topic := ""
	if len(episodes) > 0 {
		topic = episodes[0].Topic
	}

	now := time.Now()
	summary := domain.Summary{
		ID:         fmt.Sprintf("sum_%s", uuid.New().String()),
		AgentID:    c.AgentID,
		UserID:     c.UserID,
		EpisodeIDs: episodeIDs,
		Topic:      topic,
		Content:    summaryResult.Content,
		Embedding:  embedding,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	// 存储 Summary
	if err := a.storeSummary(c, summary); err != nil {
		a.logger.Warn("failed to store summary", "error", err)
		return
	}

	c.AddSummaries(summary)

	a.logger.Info("summary generated and stored",
		"id", summary.ID,
		"topic", summary.Topic,
		"episode_count", len(episodeIDs),
	)
}

// formatEpisodes 格式化 Episode 为对话文本
func (a *SummaryAction) formatEpisodes(episodes []domain.Episode) string {
	var lines []string

	for _, ep := range episodes {
		name := ep.Name
		if name == "" {
			name = ep.Role
		}
		lines = append(lines, fmt.Sprintf("%s: %s", name, ep.Content))
	}

	return strings.Join(lines, "\n")
}

// storeSummary 存储 Summary 到 OpenSearch
func (a *SummaryAction) storeSummary(c *domain.AddContext, summary domain.Summary) error {
	if a.store == nil {
		return nil
	}

	doc := map[string]any{
		"id":          summary.ID,
		"type":        domain.DocTypeSummary,
		"agent_id":    summary.AgentID,
		"user_id":     summary.UserID,
		"episode_ids": summary.EpisodeIDs,
		"topic":       summary.Topic,
		"content":     summary.Content,
		"embedding":   summary.Embedding,
		"created_at":  summary.CreatedAt,
		"updated_at":  summary.UpdatedAt,
	}

	return a.store.Store(c.Context, summary.ID, doc)
}
