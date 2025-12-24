package action

import (
	"context"
	"log/slog"
	"math"
	"time"

	"github.com/Zereker/memory/internal/domain"
	"github.com/Zereker/memory/pkg/relation"
	"github.com/Zereker/memory/pkg/vector"
)

const (
	// 遗忘阈值
	ForgetThreshold = 0.7

	// 事实记忆 ILM 过期天数
	FactExpiryDays = 90

	// 最大时间衰减天数（用于归一化）
	MaxDecayDays = 30.0
)

// ForgettingAction 记忆遗忘处理器
type ForgettingAction struct {
	logger        *slog.Logger
	vectorStore   vector.Store
	relationStore relation.Store
}

// NewForgettingAction 创建 ForgettingAction
func NewForgettingAction() *ForgettingAction {
	return &ForgettingAction{
		logger:        slog.Default().With("module", "forgetting"),
		vectorStore:   vector.NewStore(),
		relationStore: relation.NewStore(),
	}
}

// WithStores 设置存储（用于测试注入 mock）
func (a *ForgettingAction) WithStores(v vector.Store, r relation.Store) *ForgettingAction {
	a.vectorStore = v
	a.relationStore = r
	return a
}

// Execute 执行遗忘流程
func (a *ForgettingAction) Execute(ctx context.Context, agentID, userID string) (*domain.ForgetResponse, error) {
	a.logger.Info("executing forgetting", "agent_id", agentID, "user_id", userID)

	resp := &domain.ForgetResponse{Success: true}

	// 1. 遗忘工作记忆
	workingForgot, err := a.forgetWorkingMemories(ctx, agentID, userID)
	if err != nil {
		a.logger.Warn("failed to forget working memories", "error", err)
	}
	resp.WorkingForgot = workingForgot

	// 2. 遗忘事件图谱
	eventsForgot, err := a.forgetEvents(ctx, agentID, userID)
	if err != nil {
		a.logger.Warn("failed to forget events", "error", err)
	}
	resp.EventsForgot = eventsForgot

	// 3. 过期事实记忆（3 个月 ILM）
	factsExpired, err := a.expireFactMemories(ctx, agentID, userID)
	if err != nil {
		a.logger.Warn("failed to expire fact memories", "error", err)
	}
	resp.FactsExpired = factsExpired

	a.logger.Info("forgetting completed",
		"working_forgot", workingForgot,
		"events_forgot", eventsForgot,
		"facts_expired", factsExpired,
	)

	return resp, nil
}

// forgetWorkingMemories 遗忘工作记忆
// forget_score = 0.5*(1-importance) + 0.3*time_factor + 0.2*freq_factor
// > 0.7 遗忘，跳过 is_protected
func (a *ForgettingAction) forgetWorkingMemories(ctx context.Context, agentID, userID string) (int, error) {
	if a.vectorStore == nil {
		return 0, nil
	}

	docs, err := a.vectorStore.Search(ctx, vector.SearchQuery{
		Filters: map[string]any{
			"type":        domain.DocTypeSummary,
			"memory_type": domain.MemoryTypeWorking,
			"agent_id":    agentID,
			"user_id":     userID,
		},
		Limit: 1000,
	})
	if err != nil {
		return 0, err
	}

	base := NewBaseAction("forgetting")
	forgot := 0
	now := time.Now()

	// 需要类型断言来使用 Delete 方法
	type deleter interface {
		Delete(ctx context.Context, id string) error
	}
	del, canDelete := a.vectorStore.(deleter)

	for _, doc := range docs {
		s := base.DocToSummaryMemory(doc)

		// 跳过受保护的记忆
		if s.IsProtected {
			continue
		}

		score := a.calcWorkingForgetScore(s, now)
		if score > ForgetThreshold {
			if canDelete {
				if err := del.Delete(ctx, s.ID); err != nil {
					a.logger.Warn("failed to delete working memory", "id", s.ID, "error", err)
					continue
				}
			}
			forgot++
		}
	}

	return forgot, nil
}

// calcWorkingForgetScore 计算工作记忆遗忘分数
func (a *ForgettingAction) calcWorkingForgetScore(s *domain.SummaryMemory, now time.Time) float64 {
	// 重要性因子：1 - importance
	importanceFactor := 1.0 - s.Importance

	// 时间因子：距上次访问的天数 / 最大衰减天数，归一化到 [0, 1]
	daysSinceAccess := now.Sub(s.LastAccessedAt).Hours() / 24.0
	timeFactor := math.Min(daysSinceAccess/MaxDecayDays, 1.0)

	// 频率因子：访问次数越少越容易遗忘
	freqFactor := 1.0
	if s.AccessCount > 0 {
		freqFactor = 1.0 / (1.0 + math.Log(float64(s.AccessCount)))
	}

	return 0.5*importanceFactor + 0.3*timeFactor + 0.2*freqFactor
}

// forgetEvents 遗忘事件图谱
// forget_score = 0.6*time + 0.4*freq
// > 0.7 删除事件（OpenSearch 删文档 + PostgreSQL 级联删关系）
func (a *ForgettingAction) forgetEvents(ctx context.Context, agentID, userID string) (int, error) {
	if a.vectorStore == nil {
		return 0, nil
	}

	docs, err := a.vectorStore.Search(ctx, vector.SearchQuery{
		Filters: map[string]any{
			"type":     domain.DocTypeEvent,
			"agent_id": agentID,
			"user_id":  userID,
		},
		Limit: 1000,
	})
	if err != nil {
		return 0, err
	}

	base := NewBaseAction("forgetting")
	forgot := 0
	now := time.Now()

	type deleter interface {
		Delete(ctx context.Context, id string) error
	}
	del, canDelete := a.vectorStore.(deleter)

	for _, doc := range docs {
		e := base.DocToEventTriplet(doc)

		score := a.calcEventForgetScore(e, now)
		if score > ForgetThreshold {
			// 从 OpenSearch 删除
			if canDelete {
				if err := del.Delete(ctx, e.ID); err != nil {
					a.logger.Warn("failed to delete event from vector", "id", e.ID, "error", err)
				}
			}

			// 从 PostgreSQL 删除关联的关系
			if a.relationStore != nil {
				if err := a.relationStore.DeleteByEventID(ctx, e.ID); err != nil {
					a.logger.Warn("failed to delete event relations", "id", e.ID, "error", err)
				}
			}

			forgot++
		}
	}

	return forgot, nil
}

// calcEventForgetScore 计算事件遗忘分数
func (a *ForgettingAction) calcEventForgetScore(e *domain.EventTriplet, now time.Time) float64 {
	daysSinceAccess := now.Sub(e.LastAccessedAt).Hours() / 24.0
	timeFactor := math.Min(daysSinceAccess/MaxDecayDays, 1.0)

	freqFactor := 1.0
	if e.AccessCount > 0 {
		freqFactor = 1.0 / (1.0 + math.Log(float64(e.AccessCount)))
	}

	return 0.6*timeFactor + 0.4*freqFactor
}

// expireFactMemories 过期事实记忆（3 个月 ILM）
func (a *ForgettingAction) expireFactMemories(ctx context.Context, agentID, userID string) (int, error) {
	if a.vectorStore == nil {
		return 0, nil
	}

	cutoff := time.Now().AddDate(0, 0, -FactExpiryDays)

	docs, err := a.vectorStore.Search(ctx, vector.SearchQuery{
		Filters: map[string]any{
			"type":        domain.DocTypeSummary,
			"memory_type": domain.MemoryTypeFact,
			"agent_id":    agentID,
			"user_id":     userID,
		},
		RangeFilters: map[string]map[string]any{
			"created_at": {"lt": cutoff.Format(time.RFC3339)},
		},
		Limit: 1000,
	})
	if err != nil {
		return 0, err
	}

	base := NewBaseAction("forgetting")
	expired := 0

	type deleter interface {
		Delete(ctx context.Context, id string) error
	}
	del, canDelete := a.vectorStore.(deleter)

	for _, doc := range docs {
		s := base.DocToSummaryMemory(doc)

		// 跳过受保护的
		if s.IsProtected {
			continue
		}

		if canDelete {
			if err := del.Delete(ctx, s.ID); err != nil {
				a.logger.Warn("failed to delete expired fact", "id", s.ID, "error", err)
				continue
			}
		}
		expired++
	}

	return expired, nil
}
