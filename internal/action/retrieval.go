package action

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Zereker/memory/internal/domain"
	"github.com/Zereker/memory/pkg/vector"
)

// 默认预算配置
const (
	DefaultMaxTokens = 2000 // 总 token 预算

	// 3-Bucket 默认配额
	DefaultFactTokens    = 1000 // Fact 桶 50%
	DefaultGraphTokens   = 400  // Graph 桶 20%（保底 400）
	DefaultWorkingTokens = 600  // Working 桶 30%

	// Graph 桶保底
	GraphMinTokens = 400

	// token 估算系数（中文约 1.5 字符/token）
	CharsPerToken = 1.5
)

// 确保实现 domain.RecallAction 接口
var _ domain.RecallAction = (*CognitiveRetrievalAction)(nil)

// CognitiveRetrievalAction 认知检索 Action
// 使用 3-Bucket Token 预算分配策略
type CognitiveRetrievalAction struct {
	*BaseAction

	vectorStore vector.Store
}

// NewCognitiveRetrievalAction 创建 CognitiveRetrievalAction
func NewCognitiveRetrievalAction() *CognitiveRetrievalAction {
	return &CognitiveRetrievalAction{
		BaseAction:  NewBaseAction("cognitive_retrieval"),
		vectorStore: vector.NewStore(),
	}
}

// WithStores 设置存储（用于测试注入 mock）
func (a *CognitiveRetrievalAction) WithStores(v vector.Store) *CognitiveRetrievalAction {
	a.vectorStore = v
	return a
}

// Name 返回 action 名称
func (a *CognitiveRetrievalAction) Name() string {
	return "cognitive_retrieval"
}

// tokenBudget 管理 3-Bucket token 预算
type tokenBudget struct {
	total   int
	fact    int // Fact 桶配额
	graph   int // Graph 桶配额
	working int // Working 桶配额

	factUsed    int
	graphUsed   int
	workingUsed int
}

// HandleRecall 执行认知检索
// 排列策略：Fact(顶) → Working(中) → Graph(中后) → ShortTerm(底) (Lost in Middle)
func (a *CognitiveRetrievalAction) HandleRecall(c *domain.RecallContext) {
	a.logger.Info("executing", "query", c.Query, "limit", c.Limit)

	// 1. 生成查询向量
	embedding, err := a.GenEmbedding(c.Context, EmbedderName, c.Query)
	if err != nil {
		a.logger.Error("failed to generate query embedding", "error", err)
		c.Next()
		return
	}
	c.Embedding = embedding

	// 2. 初始化 3-Bucket 预算
	budget := a.initBudget(c)

	// 3. Step 1: 保底填充 Graph 桶（强制 400 tokens）
	a.searchEvents(c, budget)

	// 4. Step 2: 优先级贪婪填充
	// Fact 桶
	a.searchFactMemories(c, budget)

	// Working 桶
	a.searchWorkingMemories(c, budget)

	// 5. Step 3: 未用空间再分配
	a.redistributeUnused(c, budget)

	// 6. 异步更新 access_count 和 last_accessed_at
	go a.updateAccessStats(c)

	a.logger.Info("cognitive retrieval completed",
		"facts", len(c.Facts),
		"working", len(c.WorkingMem),
		"events", len(c.Events),
		"tokens_fact", budget.factUsed,
		"tokens_graph", budget.graphUsed,
		"tokens_working", budget.workingUsed,
	)

	c.Next()
}

// initBudget 初始化 3-Bucket 预算
func (a *CognitiveRetrievalAction) initBudget(c *domain.RecallContext) *tokenBudget {
	budget := &tokenBudget{
		total:   DefaultMaxTokens,
		fact:    DefaultFactTokens,
		graph:   DefaultGraphTokens,
		working: DefaultWorkingTokens,
	}

	if c.Options.MaxTokens > 0 {
		budget.total = c.Options.MaxTokens
		// 按比例重新分配
		budget.fact = budget.total * 50 / 100
		budget.graph = budget.total * 20 / 100
		budget.working = budget.total * 30 / 100
	}

	// Graph 桶保底
	if budget.graph < GraphMinTokens {
		budget.graph = GraphMinTokens
	}

	// 自定义覆盖
	if c.Options.MaxFacts > 0 {
		budget.fact = c.Options.MaxFacts
	} else if c.Options.MaxFacts < 0 {
		budget.fact = 0
	}

	if c.Options.MaxGraph > 0 {
		budget.graph = c.Options.MaxGraph
	} else if c.Options.MaxGraph < 0 {
		budget.graph = 0
	}

	if c.Options.MaxWorking > 0 {
		budget.working = c.Options.MaxWorking
	} else if c.Options.MaxWorking < 0 {
		budget.working = 0
	}

	return budget
}

// searchFactMemories 检索 fact 类型记忆
func (a *CognitiveRetrievalAction) searchFactMemories(c *domain.RecallContext, budget *tokenBudget) {
	if a.vectorStore == nil || budget.fact <= 0 {
		return
	}

	docs, err := a.vectorStore.Search(c.Context, vector.SearchQuery{
		Embedding: c.Embedding,
		Filters: map[string]any{
			"type":        domain.DocTypeSummary,
			"memory_type": domain.MemoryTypeFact,
			"agent_id":    c.AgentID,
			"user_id":     c.UserID,
		},
		Limit: c.Limit,
	})
	if err != nil {
		a.logger.Warn("fact search failed", "error", err)
		return
	}

	for _, doc := range docs {
		s := a.DocToSummaryMemory(doc)
		if score, ok := doc["_score"].(float64); ok {
			s.Score = score
		}

		tokens := estimateTokens(s.Content)
		if budget.factUsed+tokens > budget.fact {
			break
		}

		c.Facts = append(c.Facts, *s)
		budget.factUsed += tokens
	}
}

// searchWorkingMemories 检索 working 类型记忆
func (a *CognitiveRetrievalAction) searchWorkingMemories(c *domain.RecallContext, budget *tokenBudget) {
	if a.vectorStore == nil || budget.working <= 0 {
		return
	}

	docs, err := a.vectorStore.Search(c.Context, vector.SearchQuery{
		Embedding: c.Embedding,
		Filters: map[string]any{
			"type":        domain.DocTypeSummary,
			"memory_type": domain.MemoryTypeWorking,
			"agent_id":    c.AgentID,
			"user_id":     c.UserID,
		},
		Limit: c.Limit,
	})
	if err != nil {
		a.logger.Warn("working memory search failed", "error", err)
		return
	}

	for _, doc := range docs {
		s := a.DocToSummaryMemory(doc)
		if score, ok := doc["_score"].(float64); ok {
			s.Score = score
		}

		tokens := estimateTokens(s.Content)
		if budget.workingUsed+tokens > budget.working {
			break
		}

		c.WorkingMem = append(c.WorkingMem, *s)
		budget.workingUsed += tokens
	}
}

// searchEvents 检索事件三元组
func (a *CognitiveRetrievalAction) searchEvents(c *domain.RecallContext, budget *tokenBudget) {
	if a.vectorStore == nil || budget.graph <= 0 {
		return
	}

	// 从 OpenSearch 用触发词向量检索
	docs, err := a.vectorStore.Search(c.Context, vector.SearchQuery{
		Embedding: c.Embedding,
		Filters: map[string]any{
			"type":     domain.DocTypeEvent,
			"agent_id": c.AgentID,
			"user_id":  c.UserID,
		},
		Limit: c.Limit,
	})
	if err != nil {
		a.logger.Warn("event search failed", "error", err)
		return
	}

	for _, doc := range docs {
		e := a.DocToEventTriplet(doc)
		if score, ok := doc["_score"].(float64); ok {
			e.Score = score
		}

		eventText := e.Argument1 + e.TriggerWord + e.Argument2
		tokens := estimateTokens(eventText)
		if budget.graphUsed+tokens > budget.graph {
			break
		}

		c.Events = append(c.Events, *e)
		budget.graphUsed += tokens
	}
}

// redistributeUnused 将未用空间再分配
func (a *CognitiveRetrievalAction) redistributeUnused(c *domain.RecallContext, budget *tokenBudget) {
	// 计算各桶剩余
	factRemain := budget.fact - budget.factUsed
	graphRemain := budget.graph - budget.graphUsed
	workingRemain := budget.working - budget.workingUsed
	totalRemain := factRemain + graphRemain + workingRemain

	if totalRemain <= 0 {
		return
	}

	// 将剩余 token 按优先级分配给不足的桶
	// 优先补充 Fact
	if factRemain > 0 && a.vectorStore != nil {
		a.searchMoreFactMemories(c, budget, factRemain+graphRemain+workingRemain)
	}
}

// searchMoreFactMemories 使用剩余预算搜索更多 fact 记忆
func (a *CognitiveRetrievalAction) searchMoreFactMemories(c *domain.RecallContext, budget *tokenBudget, extraBudget int) {
	if a.vectorStore == nil || extraBudget <= 0 {
		return
	}

	// 搜索更多（跳过已有的）
	docs, err := a.vectorStore.Search(c.Context, vector.SearchQuery{
		Embedding: c.Embedding,
		Filters: map[string]any{
			"type":        domain.DocTypeSummary,
			"memory_type": domain.MemoryTypeFact,
			"agent_id":    c.AgentID,
			"user_id":     c.UserID,
		},
		Limit: c.Limit * 2,
	})
	if err != nil {
		return
	}

	seen := make(map[string]bool, len(c.Facts))
	for _, f := range c.Facts {
		seen[f.ID] = true
	}

	used := 0
	for _, doc := range docs {
		s := a.DocToSummaryMemory(doc)
		if seen[s.ID] {
			continue
		}
		if score, ok := doc["_score"].(float64); ok {
			s.Score = score
		}

		tokens := estimateTokens(s.Content)
		if used+tokens > extraBudget {
			break
		}

		c.Facts = append(c.Facts, *s)
		used += tokens
	}
}

// updateAccessStats 异步更新访问统计
func (a *CognitiveRetrievalAction) updateAccessStats(c *domain.RecallContext) {
	// 通过类型断言获取 OpenSearchStore 的 UpdateFields 能力
	type fieldUpdater interface {
		UpdateFields(ctx context.Context, id string, fields map[string]any) error
	}

	updater, ok := a.vectorStore.(fieldUpdater)
	if !ok {
		return
	}

	now := time.Now()

	// 更新 fact 记忆
	for _, f := range c.Facts {
		_ = updater.UpdateFields(c.Context, f.ID, map[string]any{
			"access_count":     f.AccessCount + 1,
			"last_accessed_at": now,
		})
	}

	// 更新 working 记忆
	for _, w := range c.WorkingMem {
		_ = updater.UpdateFields(c.Context, w.ID, map[string]any{
			"access_count":     w.AccessCount + 1,
			"last_accessed_at": now,
		})
	}

	// 更新事件
	for _, e := range c.Events {
		_ = updater.UpdateFields(c.Context, e.ID, map[string]any{
			"access_count":     e.AccessCount + 1,
			"last_accessed_at": now,
		})
	}
}

// FormatMemoryContext 将检索结果格式化为 LLM prompt
// 排列顺序：Fact(顶) → Working(中) → Graph(中后) → ShortTerm(底) (Lost in Middle 策略)
func FormatMemoryContext(c *domain.RecallContext) string {
	var parts []string

	// Fact 记忆（顶部）
	if len(c.Facts) > 0 {
		parts = append(parts, "## 用户事实")
		for _, f := range c.Facts {
			ts := f.CreatedAt.Format("2006-01-02")
			parts = append(parts, fmt.Sprintf("- [%s] %s", ts, f.Content))
		}
	}

	// Working 记忆（中部）
	if len(c.WorkingMem) > 0 {
		parts = append(parts, "\n## 工作记忆")
		for _, w := range c.WorkingMem {
			ts := w.CreatedAt.Format("2006-01-02")
			parts = append(parts, fmt.Sprintf("- [%s] %s", ts, w.Content))
		}
	}

	// 事件图谱（中后部）
	if len(c.Events) > 0 {
		parts = append(parts, "\n## 相关事件")
		for _, e := range c.Events {
			ts := e.CreatedAt.Format("2006-01-02")
			parts = append(parts, fmt.Sprintf("- [%s] %s %s %s", ts, e.Argument1, e.TriggerWord, e.Argument2))
		}
	}

	// 短期记忆（底部）
	if len(c.ShortTerm) > 0 {
		parts = append(parts, "\n## 近期对话")
		for _, msg := range c.ShortTerm {
			name := msg.Name
			if name == "" {
				name = msg.Role
			}
			parts = append(parts, fmt.Sprintf("- [%s] %s", name, msg.Content))
		}
	}

	if len(parts) == 0 {
		return "没有找到相关的记忆信息。"
	}

	return strings.Join(parts, "\n")
}

// estimateTokens 估算文本的 token 数量
func estimateTokens(text string) int {
	charCount := utf8.RuneCountInString(text)
	return int(float64(charCount) / CharsPerToken)
}

