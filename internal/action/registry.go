package action

import (
	"context"
	"log/slog"

	"github.com/Zereker/memory/internal/domain"
)

// Memory 统一的记忆操作入口
type Memory struct {
	logger     *slog.Logger
	forgetting *ForgettingAction
}

// NewMemory 创建 Memory 实例
func NewMemory() *Memory {
	return &Memory{
		logger:     slog.Default().With("module", "memory"),
		forgetting: NewForgettingAction(),
	}
}

// Add 从对话中添加记忆
// Chain: ShortTermAction → SummaryMemoryAction → EventExtractionAction → ConsistencyAction
func (m *Memory) Add(ctx context.Context, req *domain.AddRequest) (*domain.AddResponse, error) {
	userID, agentID := inferUserAndAgent(req)

	m.logger.Info("add",
		"agent_id", agentID,
		"user_id", userID,
		"session_id", req.SessionID,
		"message_count", len(req.Messages),
	)

	// 创建 action chain
	chain := domain.NewActionChain()
	chain.Use(NewShortTermAction())         // 1. 短期记忆窗口
	chain.Use(NewSummaryMemoryAction())     // 2. 摘要记忆提取
	chain.Use(NewEventExtractionAction())   // 3. 事件三元组提取
	chain.Use(NewConsistencyAction())       // 4. 认知一致性检查

	// 创建 context
	addCtx := domain.NewAddContext(ctx, agentID, userID, req.SessionID)
	addCtx.Messages = domain.Messages(req.Messages)

	// 执行 chain
	chain.Run(addCtx)

	// 构建响应
	resp := &domain.AddResponse{
		Success:        true,
		Summaries:      addCtx.Summaries,
		Events:         addCtx.Events,
		EventRelations: addCtx.EventRelations,
	}

	m.logger.Info("add completed",
		"summaries", len(resp.Summaries),
		"events", len(resp.Events),
		"relations", len(resp.EventRelations),
	)

	return resp, nil
}

// Retrieve 检索相关记忆
// Chain: ShortTermRecallAction → CognitiveRetrievalAction
func (m *Memory) Retrieve(ctx context.Context, req *domain.RetrieveRequest) (*domain.RetrieveResponse, error) {
	m.logger.Info("retrieve",
		"agent_id", req.AgentID,
		"user_id", req.UserID,
		"query", req.Query,
	)

	// 创建 recall chain
	chain := domain.NewRecallChain()
	chain.Use(NewShortTermRecallAction())     // 1. 短期记忆召回
	chain.Use(NewCognitiveRetrievalAction())  // 2. 认知检索

	// 创建 context
	recallCtx := domain.NewRecallContext(ctx, req)

	// 执行 chain
	chain.Run(recallCtx)

	// 构建响应
	resp := &domain.RetrieveResponse{
		Success:    true,
		Facts:      recallCtx.Facts,
		WorkingMem: recallCtx.WorkingMem,
		Events:     recallCtx.Events,
		ShortTerm:  recallCtx.ShortTerm,
		Total:      recallCtx.TotalResults(),
	}

	// 格式化记忆上下文
	resp.MemoryContext = FormatMemoryContext(recallCtx)

	m.logger.Info("retrieve completed",
		"facts", len(resp.Facts),
		"working", len(resp.WorkingMem),
		"events", len(resp.Events),
		"short_term", len(resp.ShortTerm),
		"total", resp.Total,
	)

	return resp, nil
}

// Forget 执行记忆遗忘
func (m *Memory) Forget(ctx context.Context, req *domain.ForgetRequest) (*domain.ForgetResponse, error) {
	m.logger.Info("forget",
		"agent_id", req.AgentID,
		"user_id", req.UserID,
	)

	return m.forgetting.Execute(ctx, req.AgentID, req.UserID)
}

// Delete 删除记忆
func (m *Memory) Delete(ctx context.Context, id string) error {
	m.logger.Info("delete", "id", id)
	// TODO: 实现删除逻辑
	return nil
}

// inferUserAndAgent 从请求和 messages 中推断 user_id 和 agent_id
func inferUserAndAgent(req *domain.AddRequest) (userID, agentID string) {
	userID = req.UserID
	agentID = req.AgentID

	if userID != "" && agentID != "" {
		return userID, agentID
	}

	messages := domain.Messages(req.Messages)
	if userID == "" {
		userID = messages.UserName()
	}
	if agentID == "" {
		agentID = messages.AssistantName()
	}

	return userID, agentID
}
