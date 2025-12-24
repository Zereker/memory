package action

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Zereker/memory/internal/domain"
	"github.com/Zereker/memory/pkg/vector"
)

// 确保实现 domain.AddAction 接口
var _ domain.AddAction = (*SummaryMemoryAction)(nil)

// SummaryMemoryAction 摘要记忆提取 Action（Layer 2）
// LLM 单次调用输出：content + importance + memory_type + keywords
type SummaryMemoryAction struct {
	*BaseAction
	store vector.Store
}

// NewSummaryMemoryAction 创建 SummaryMemoryAction
func NewSummaryMemoryAction() *SummaryMemoryAction {
	return &SummaryMemoryAction{
		BaseAction: NewBaseAction("summary_memory"),
		store:      vector.NewStore(),
	}
}

// WithStore 设置存储（用于测试注入 mock）
func (a *SummaryMemoryAction) WithStore(store vector.Store) *SummaryMemoryAction {
	a.store = store
	return a
}

// Name 返回 action 名称
func (a *SummaryMemoryAction) Name() string {
	return "summary_memory"
}

// MemoryExtractResult LLM 提取结果
type MemoryExtractResult struct {
	Memories []ExtractedMemory `json:"memories"`
}

// ExtractedMemory 单条提取的记忆
type ExtractedMemory struct {
	Content    string   `json:"content"`
	Importance float64  `json:"importance"`
	MemoryType string   `json:"memory_type"`
	Keywords   []string `json:"keywords"`
}

// Handle 执行摘要记忆提取
func (a *SummaryMemoryAction) Handle(c *domain.AddContext) {
	a.logger.Info("executing", "session_id", c.SessionID, "message_count", len(c.Messages))

	if len(c.Messages) == 0 {
		c.Next()
		return
	}

	// 调用 LLM 提取记忆
	conversation := c.Messages.Format()
	var result MemoryExtractResult
	if err := a.Generate(c, "memory_extract", map[string]any{
		"conversation": conversation,
		"language":     c.LanguageName(),
	}, &result); err != nil {
		a.logger.Error("memory extraction failed", "error", err)
		c.Next()
		return
	}

	if len(result.Memories) == 0 {
		a.logger.Debug("no memories extracted")
		c.Next()
		return
	}

	now := time.Now()
	for _, mem := range result.Memories {
		// 生成 embedding
		embedding, err := a.GenEmbedding(c.Context, EmbedderName, mem.Content)
		if err != nil {
			a.logger.Warn("failed to generate embedding", "error", err)
			continue
		}

		// importance >= 0.9 自动标记保护
		isProtected := mem.Importance >= 0.9

		summary := domain.SummaryMemory{
			ID:             fmt.Sprintf("mem_%s", uuid.New().String()[:8]),
			AgentID:        c.AgentID,
			UserID:         c.UserID,
			Content:        mem.Content,
			MemoryType:     mem.MemoryType,
			Importance:     mem.Importance,
			Keywords:       mem.Keywords,
			Embedding:      embedding,
			IsProtected:    isProtected,
			AccessCount:    0,
			LastAccessedAt: now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		// 存储到 OpenSearch
		if err := a.storeSummary(c, summary); err != nil {
			a.logger.Warn("failed to store summary", "id", summary.ID, "error", err)
			continue
		}

		c.AddSummaries(summary)
	}

	a.logger.Info("summary memories extracted",
		"count", len(c.Summaries),
	)

	c.Next()
}

// storeSummary 存储摘要记忆到 OpenSearch
func (a *SummaryMemoryAction) storeSummary(c *domain.AddContext, s domain.SummaryMemory) error {
	if a.store == nil {
		return nil
	}

	doc := map[string]any{
		"id":               s.ID,
		"type":             domain.DocTypeSummary,
		"agent_id":         s.AgentID,
		"user_id":          s.UserID,
		"content":          s.Content,
		"memory_type":      s.MemoryType,
		"importance":       s.Importance,
		"keywords":         s.Keywords,
		"embedding":        s.Embedding,
		"is_protected":     s.IsProtected,
		"access_count":     s.AccessCount,
		"last_accessed_at": s.LastAccessedAt,
		"created_at":       s.CreatedAt,
		"updated_at":       s.UpdatedAt,
	}

	return a.store.Store(c.Context, s.ID, doc)
}
