package action

import (
	"sync"
	"time"

	"github.com/Zereker/memory/internal/domain"
)

const (
	// DefaultWindowSize 默认滑动窗口大小（10 轮对话 = 20 条消息）
	DefaultWindowSize = 20
)

// ShortTermStore 短期记忆存储（内存滑动窗口）
type ShortTermStore struct {
	mu         sync.RWMutex
	windows    map[string]*domain.ShortTermMemory // key: agentID:userID:sessionID
	windowSize int
}

// 全局短期记忆存储
var shortTermStore = &ShortTermStore{
	windows:    make(map[string]*domain.ShortTermMemory),
	windowSize: DefaultWindowSize,
}

// GetShortTermStore 获取全局短期记忆存储
func GetShortTermStore() *ShortTermStore {
	return shortTermStore
}

// windowKey 生成窗口 key
func windowKey(agentID, userID, sessionID string) string {
	return agentID + ":" + userID + ":" + sessionID
}

// GetWindow 获取指定会话的短期记忆窗口
func (s *ShortTermStore) GetWindow(agentID, userID, sessionID string) *domain.ShortTermMemory {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := windowKey(agentID, userID, sessionID)
	if w, ok := s.windows[key]; ok {
		return w
	}
	return nil
}

// AppendMessages 追加消息到窗口（自动滑动）
func (s *ShortTermStore) AppendMessages(agentID, userID, sessionID string, messages domain.Messages) *domain.ShortTermMemory {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := windowKey(agentID, userID, sessionID)
	w, ok := s.windows[key]
	if !ok {
		w = &domain.ShortTermMemory{
			AgentID:   agentID,
			UserID:    userID,
			SessionID: sessionID,
		}
		s.windows[key] = w
	}

	w.Messages = append(w.Messages, messages...)
	w.UpdatedAt = time.Now()

	// 滑动窗口：保留最近的消息
	if len(w.Messages) > s.windowSize {
		w.Messages = w.Messages[len(w.Messages)-s.windowSize:]
	}

	return w
}

// Clear 清除指定会话的短期记忆
func (s *ShortTermStore) Clear(agentID, userID, sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.windows, windowKey(agentID, userID, sessionID))
}

// ============================================================================
// ShortTermAction - 写入时追加消息到滑动窗口
// ============================================================================

var _ domain.AddAction = (*ShortTermAction)(nil)

// ShortTermAction 短期记忆写入 Action
type ShortTermAction struct {
	*BaseAction
	store *ShortTermStore
}

// NewShortTermAction 创建 ShortTermAction
func NewShortTermAction() *ShortTermAction {
	return &ShortTermAction{
		BaseAction: NewBaseAction("short_term"),
		store:      GetShortTermStore(),
	}
}

// Name 返回 action 名称
func (a *ShortTermAction) Name() string {
	return "short_term"
}

// Handle 追加消息到短期记忆窗口
func (a *ShortTermAction) Handle(c *domain.AddContext) {
	if len(c.Messages) == 0 {
		c.Next()
		return
	}

	w := a.store.AppendMessages(c.AgentID, c.UserID, c.SessionID, c.Messages)
	c.ShortTermWindow = w

	a.logger.Info("short term window updated",
		"session_id", c.SessionID,
		"window_size", len(w.Messages),
	)

	c.Next()
}

// ============================================================================
// ShortTermRecallAction - 召回时获取窗口内容
// ============================================================================

var _ domain.RecallAction = (*ShortTermRecallAction)(nil)

// ShortTermRecallAction 短期记忆召回 Action
type ShortTermRecallAction struct {
	*BaseAction
	store *ShortTermStore
}

// NewShortTermRecallAction 创建 ShortTermRecallAction
func NewShortTermRecallAction() *ShortTermRecallAction {
	return &ShortTermRecallAction{
		BaseAction: NewBaseAction("short_term_recall"),
		store:      GetShortTermStore(),
	}
}

// Name 返回 action 名称
func (a *ShortTermRecallAction) Name() string {
	return "short_term_recall"
}

// HandleRecall 获取短期记忆窗口内容
func (a *ShortTermRecallAction) HandleRecall(c *domain.RecallContext) {
	w := a.store.GetWindow(c.AgentID, c.UserID, c.SessionID)
	if w != nil && len(w.Messages) > 0 {
		c.ShortTerm = w.Messages
		a.logger.Info("short term recall",
			"session_id", c.SessionID,
			"messages", len(c.ShortTerm),
		)
	}

	c.Next()
}
