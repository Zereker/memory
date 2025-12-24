package action

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/Zereker/memory/internal/domain"
	"github.com/Zereker/memory/pkg/storage"
)

// 确保实现 domain.AddAction 接口
var _ domain.AddAction = (*EpisodeStorageAction)(nil)

// EpisodeStorageAction 将原始对话存储为 Episode
type EpisodeStorageAction struct {
	*BaseAction

	vectorStore *storage.OpenSearchStore
}

// NewEpisodeStorageAction 创建 EpisodeStorageAction
func NewEpisodeStorageAction() *EpisodeStorageAction {
	return &EpisodeStorageAction{
		BaseAction:  NewBaseAction("episode_storage"),
		vectorStore: storage.NewStore(),
	}
}

// Name 返回 action 名称
func (a *EpisodeStorageAction) Name() string {
	return "episode_storage"
}

// TopicResult topic prompt 输出
type TopicResult struct {
	Topic string `json:"topic"`
}

// Handle 执行 Episode 存储
func (a *EpisodeStorageAction) Handle(c *domain.AddContext) {
	a.logger.Info("executing", "session_id", c.SessionID, "message_count", len(c.Messages))

	if len(c.Messages) == 0 {
		a.logger.Debug("no messages, skipping")
		c.Next()
		return
	}

	now := time.Now()

	// 将每条消息转换为 Episode
	for i, msg := range c.Messages {
		// 检查 context 是否已取消
		if c.Context.Err() != nil {
			c.SetError(errors.Wrap(c.Context.Err(), "context cancelled"))
			return
		}

		embedding, err := a.GenEmbedding(c.Context, EmbedderName, msg.Content)
		if err != nil {
			a.logger.Warn("failed to generate embedding", "index", i, "error", err)
			continue
		}

		var topicResult TopicResult
		if err := a.Generate(c, "topic", map[string]any{
			"content":  msg.Content,
			"language": c.LanguageName(),
		}, &topicResult); err != nil {
			a.logger.Warn("failed to generate topic", "index", i, "error", err)
			continue
		}

		// 生成 topic embedding
		topicEmbedding, err := a.GenEmbedding(c.Context, EmbedderName, topicResult.Topic)
		if err != nil {
			a.logger.Warn("failed to generate topic embedding", "index", i, "error", err)
			continue
		}

		episode := domain.Episode{
			ID:             fmt.Sprintf("ep_%s", uuid.New().String()[:8]),
			AgentID:        c.AgentID,
			UserID:         c.UserID,
			SessionID:      c.SessionID,
			Role:           msg.Role,
			Name:           msg.Name,
			Content:        msg.Content,
			Embedding:      embedding,
			Topic:          topicResult.Topic,
			TopicEmbedding: topicEmbedding,
			Timestamp:      now,
			CreatedAt:      now,
		}

		if err := a.storeEpisode(c, episode); err != nil {
			a.logger.Warn("failed to store episode", "id", episode.ID, "error", err)
			continue
		}

		c.AddEpisodes(episode)
	}

	a.logger.Info("episodes stored", "count", len(c.Episodes))
	c.Next()
}

// storeEpisode 存储 Episode 到 OpenSearch
func (a *EpisodeStorageAction) storeEpisode(c *domain.AddContext, ep domain.Episode) error {
	if a.vectorStore == nil {
		return errors.New("vector store not initialized")
	}

	doc := map[string]any{
		"id":              ep.ID,
		"type":            domain.DocTypeEpisode,
		"agent_id":        ep.AgentID,
		"user_id":         ep.UserID,
		"session_id":      ep.SessionID,
		"role":            ep.Role,
		"name":            ep.Name,
		"content":         ep.Content,
		"topic":           ep.Topic,
		"topic_embedding": ep.TopicEmbedding,
		"embedding":       ep.Embedding,
		"timestamp":       ep.Timestamp,
		"created_at":      ep.CreatedAt,
	}

	return a.vectorStore.Store(c.Context, ep.ID, doc)
}
