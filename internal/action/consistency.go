package action

import (
	"context"
	"time"

	"github.com/Zereker/memory/internal/domain"
	"github.com/Zereker/memory/pkg/vector"
)

// 确保实现 domain.AddAction 接口
var _ domain.AddAction = (*ConsistencyAction)(nil)

// ConsistencyAction 认知一致性检查 Action
// 写入阶段：新写入的 fact 记忆，按 keyword + embedding 搜索已有 fact
// 发现冲突则 soft-disable 旧记忆（设 expired_at）
type ConsistencyAction struct {
	*BaseAction
	store vector.Store
}

// NewConsistencyAction 创建 ConsistencyAction
func NewConsistencyAction() *ConsistencyAction {
	return &ConsistencyAction{
		BaseAction: NewBaseAction("consistency"),
		store:      vector.NewStore(),
	}
}

// WithStore 设置存储（用于测试注入 mock）
func (a *ConsistencyAction) WithStore(store vector.Store) *ConsistencyAction {
	a.store = store
	return a
}

// Name 返回 action 名称
func (a *ConsistencyAction) Name() string {
	return "consistency"
}

// Handle 执行一致性检查
// 仅处理高重要性 fact 记忆，异步执行不阻塞
func (a *ConsistencyAction) Handle(c *domain.AddContext) {
	// 筛选高重要性的 fact 记忆
	var highImportanceFacts []domain.SummaryMemory
	for _, s := range c.Summaries {
		if s.MemoryType == domain.MemoryTypeFact && s.Importance >= 0.7 {
			highImportanceFacts = append(highImportanceFacts, s)
		}
	}

	if len(highImportanceFacts) == 0 {
		c.Next()
		return
	}

	// 异步执行冲突检测，不阻塞主链
	go a.detectConflicts(c.Context, c.AgentID, c.UserID, highImportanceFacts)

	c.Next()
}

// detectConflicts 检测并处理冲突的 fact 记忆
func (a *ConsistencyAction) detectConflicts(ctx context.Context, agentID, userID string, newFacts []domain.SummaryMemory) {
	if a.store == nil {
		return
	}

	for _, newFact := range newFacts {
		if len(newFact.Embedding) == 0 {
			continue
		}

		// 搜索相似的已有 fact 记忆
		docs, err := a.store.Search(ctx, vector.SearchQuery{
			Embedding: newFact.Embedding,
			Filters: map[string]any{
				"type":        domain.DocTypeSummary,
				"memory_type": domain.MemoryTypeFact,
				"agent_id":    agentID,
				"user_id":     userID,
			},
			ScoreThreshold: 0.8, // 高相似度阈值
			Limit:          5,
		})
		if err != nil {
			a.logger.Warn("conflict search failed", "error", err)
			continue
		}

		base := NewBaseAction("consistency")
		now := time.Now()

		for _, doc := range docs {
			existing := base.DocToSummaryMemory(doc)

			// 跳过自己
			if existing.ID == newFact.ID {
				continue
			}

			// 跳过已过期的
			if existing.ExpiredAt != nil {
				continue
			}

			// 发现冲突：soft-disable 旧记忆
			a.logger.Info("conflict detected",
				"new_id", newFact.ID,
				"old_id", existing.ID,
				"new_content", newFact.Content,
				"old_content", existing.Content,
			)

			// 通过类型断言使用 UpdateFields
			type fieldUpdater interface {
				UpdateFields(ctx context.Context, id string, fields map[string]any) error
			}

			if updater, ok := a.store.(fieldUpdater); ok {
				if err := updater.UpdateFields(ctx, existing.ID, map[string]any{
					"expired_at": now,
				}); err != nil {
					a.logger.Warn("failed to expire old fact", "id", existing.ID, "error", err)
				}
			}
		}
	}
}
