package action

import (
	"context"
	"log/slog"

	"github.com/firebase/genkit/go/genkit"

	"github.com/Zereker/memory/internal/domain"
	pkggenkit "github.com/Zereker/memory/pkg/genkit"
)

// Memory 统一的记忆操作入口
type Memory struct {
	logger *slog.Logger
	g      *genkit.Genkit
}

// NewMemory 创建 Memory 实例
func NewMemory() *Memory {
	return &Memory{
		logger: slog.Default().With("module", "memory"),
		g:      pkggenkit.Genkit(),
	}
}

// Add 从对话中添加记忆
// Zep 风格处理流程 (每个 Action 负责自己的持久化):
// 1. EpisodeStorageAction - 创建 + 存储 Episodes (OpenSearch)
// 2. ExtractionAction - 提取 + 存储 Entity/Edge (Neo4j)
// 3. TopicDetectionAction - 检测主题变化 -> 触发 MQ (异步生成摘要)
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
	chain.Use(NewEpisodeStorageAction()) // 1. Episodes -> OpenSearch
	chain.Use(NewExtractionAction())     // 2. Entity/Edge -> Neo4j
	chain.Use(NewSummaryAction())        // 3. 主题检测 + 摘要生成

	// 创建 context
	addCtx := domain.NewAddContext(ctx, agentID, userID, req.SessionID)
	addCtx.Messages = domain.Messages(req.Messages)

	// 执行 chain
	chain.Run(addCtx)

	// 构建响应
	resp := &domain.AddResponse{
		Success:   true,
		Episodes:  addCtx.Episodes,
		Entities:  addCtx.Entities,
		Edges:     addCtx.Edges,
		Summaries: addCtx.Summaries,
	}

	m.logger.Info("add completed",
		"episodes", len(resp.Episodes),
		"entities", len(resp.Entities),
		"edges", len(resp.Edges),
		"summaries", len(resp.Summaries),
	)

	return resp, nil
}

// Retrieve 检索相关记忆
// 支持向量检索 + 图遍历的混合检索
func (m *Memory) Retrieve(ctx context.Context, req *domain.RetrieveRequest) (*domain.RetrieveResponse, error) {
	m.logger.Info("retrieve",
		"agent_id", req.AgentID,
		"user_id", req.UserID,
		"query", req.Query,
	)

	// 创建 recall chain（默认按优先级检索所有类型，由 Max* 控制）
	chain := domain.NewRecallChain()
	chain.Use(NewRetrievalAction())

	// 创建 context
	recallCtx := domain.NewRecallContext(ctx, req)

	// 执行 chain
	chain.Run(recallCtx)

	// 构建响应
	resp := &domain.RetrieveResponse{
		Success:   true,
		Episodes:  recallCtx.Episodes,
		Entities:  recallCtx.Entities,
		Edges:     recallCtx.Edges,
		Summaries: recallCtx.Summaries,
		Total:     recallCtx.TotalResults(),
	}

	// 格式化记忆上下文
	resp.MemoryContext = FormatMemoryContext(recallCtx)

	m.logger.Info("retrieve completed",
		"episodes", len(resp.Episodes),
		"entities", len(resp.Entities),
		"edges", len(resp.Edges),
		"total", resp.Total,
	)

	return resp, nil
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
