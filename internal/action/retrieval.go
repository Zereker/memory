package action

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/Zereker/memory/internal/domain"
	"github.com/Zereker/memory/pkg/graph"
	"github.com/Zereker/memory/pkg/storage"
)

// 默认预算配置
const (
	DefaultMaxTokens    = 2000 // 总 token 预算
	DefaultMaxSummaries = 3    // Summary 最大数量
	DefaultMaxEdges     = 10   // Edge 最大数量
	DefaultMaxEntities  = 5    // Entity 最大数量
	DefaultMaxEpisodes  = 5    // Episode 最大数量

	// token 估算系数（中文约 1.5 字符/token）
	CharsPerToken = 1.5
)

// 确保实现 domain.RecallAction 接口
var _ domain.RecallAction = (*RetrievalAction)(nil)

// RetrievalAction 检索相关记忆
// 支持向量检索 + 图遍历的混合检索
type RetrievalAction struct {
	*BaseAction

	vectorStore *storage.OpenSearchStore
	graphStore  *graph.Neo4jStore
}

// NewRetrievalAction 创建 RetrievalAction
func NewRetrievalAction() *RetrievalAction {
	return &RetrievalAction{
		BaseAction:  NewBaseAction("retrieval"),
		vectorStore: storage.NewStore(),
		graphStore:  graph.NewStore(),
	}
}

// Name 返回 action 名称
func (a *RetrievalAction) Name() string {
	return "retrieval"
}

// HandleRecall 执行记忆检索
// 按优先级检索：Summary > Edge > Entity > Episode
func (a *RetrievalAction) HandleRecall(c *domain.RecallContext) {
	a.logger.Info("executing", "query", c.Query, "limit", c.Limit)

	// 1. 生成查询向量
	embedding, err := a.GenEmbedding(c.Context, EmbedderName, c.Query)
	if err != nil {
		a.logger.Error("failed to generate query embedding", "error", err)
		c.Next()
		return
	}
	c.Embedding = embedding

	// 2. 初始化预算
	budget := a.initBudget(c)

	// 3. 按优先级检索（Summary > Edge > Entity > Episode）
	// Priority 1: Summary（最高优先级，已压缩的精华）
	if budget.maxSummaries > 0 {
		a.searchSummaries(c)
		a.truncateSummaries(c, budget.maxSummaries)
		budget.used += a.estimateSummaryTokens(c.Summaries)
	}

	// Priority 2: Edge（事实关系，信息密度高）
	if budget.maxEdges > 0 && budget.remaining() > 0 {
		a.searchEdges(c)
		a.truncateEdges(c, budget.maxEdges, budget.remaining())
		budget.used += a.estimateEdgeTokens(c.Edges)
	}

	// Priority 3: Entity（实体描述）
	if budget.maxEntities > 0 && budget.remaining() > 0 {
		a.searchEntities(c)
		// 图遍历扩展
		if c.Options.MaxHops > 0 && len(c.Entities) > 0 {
			a.expandByGraphTraversal(c)
		}
		a.truncateEntities(c, budget.maxEntities, budget.remaining())
		budget.used += a.estimateEntityTokens(c.Entities)
	}

	// Priority 4: Episode（最低优先级，可能被 Summary 覆盖）
	if budget.maxEpisodes > 0 && budget.remaining() > 0 {
		a.searchEpisodes(c)
		// 过滤已被 Summary 覆盖的 Episodes
		a.filterCoveredEpisodes(c)
		a.truncateEpisodes(c, budget.maxEpisodes, budget.remaining())
		budget.used += a.estimateEpisodeTokens(c.Episodes)
	}

	a.logger.Info("retrieval completed",
		"episodes", len(c.Episodes),
		"summaries", len(c.Summaries),
		"edges", len(c.Edges),
		"entities", len(c.Entities),
		"tokens_used", budget.used,
		"tokens_total", budget.total,
	)

	c.Next()
}

// tokenBudget 管理 token 预算
type tokenBudget struct {
	total        int
	used         int
	maxSummaries int
	maxEdges     int
	maxEntities  int
	maxEpisodes  int
}

func (b *tokenBudget) remaining() int {
	if b.used >= b.total {
		return 0
	}
	return b.total - b.used
}

// initBudget 初始化预算配置
// Max* 值含义：-1 禁用，0 使用默认值，>0 自定义
func (a *RetrievalAction) initBudget(c *domain.RecallContext) *tokenBudget {
	budget := &tokenBudget{
		total:        DefaultMaxTokens,
		maxSummaries: DefaultMaxSummaries,
		maxEdges:     DefaultMaxEdges,
		maxEntities:  DefaultMaxEntities,
		maxEpisodes:  DefaultMaxEpisodes,
	}

	if c.Options.MaxTokens > 0 {
		budget.total = c.Options.MaxTokens
	}

	// -1 禁用，0 使用默认值，>0 自定义
	budget.maxSummaries = resolveLimit(c.Options.MaxSummaries, DefaultMaxSummaries)
	budget.maxEdges = resolveLimit(c.Options.MaxEdges, DefaultMaxEdges)
	budget.maxEntities = resolveLimit(c.Options.MaxEntities, DefaultMaxEntities)
	budget.maxEpisodes = resolveLimit(c.Options.MaxEpisodes, DefaultMaxEpisodes)

	return budget
}

// resolveLimit 解析限制值：-1 禁用返回 0，0 返回默认值，>0 返回自定义值
func resolveLimit(value, defaultValue int) int {
	if value < 0 {
		return 0 // 禁用
	}
	if value == 0 {
		return defaultValue
	}
	return value
}

// searchEpisodes 向量检索 Episodes
func (a *RetrievalAction) searchEpisodes(c *domain.RecallContext) {
	if a.vectorStore == nil {
		return
	}

	docs, err := a.vectorStore.Search(c.Context, storage.SearchQuery{
		Embedding: c.Embedding,
		Filters: map[string]any{
			"type":     domain.DocTypeEpisode,
			"agent_id": c.AgentID,
			"user_id":  c.UserID,
		},
		Limit: c.Limit,
	})
	if err != nil {
		a.logger.Warn("episode search failed", "error", err)
		return
	}

	for _, doc := range docs {
		ep := a.DocToEpisode(doc)
		if score, ok := doc["_score"].(float64); ok {
			ep.Score = score
		}

		c.Episodes = append(c.Episodes, *ep)
	}
}

// searchSummaries 向量检索 Summaries
func (a *RetrievalAction) searchSummaries(c *domain.RecallContext) {
	if a.vectorStore == nil {
		return
	}

	docs, err := a.vectorStore.Search(c.Context, storage.SearchQuery{
		Embedding: c.Embedding,
		Filters: map[string]any{
			"type":     domain.DocTypeSummary,
			"agent_id": c.AgentID,
			"user_id":  c.UserID,
		},
		Limit: c.Limit,
	})
	if err != nil {
		a.logger.Warn("summary search failed", "error", err)
		return
	}

	for _, doc := range docs {
		s := a.DocToSummary(doc)
		if score, ok := doc["_score"].(float64); ok {
			s.Score = score
		}

		c.Summaries = append(c.Summaries, *s)
	}
}

// searchEdges 向量检索 Edges
func (a *RetrievalAction) searchEdges(c *domain.RecallContext) {
	if a.vectorStore == nil {
		return
	}

	docs, err := a.vectorStore.Search(c.Context, storage.SearchQuery{
		Embedding: c.Embedding,
		Filters: map[string]any{
			"type":     domain.DocTypeEdge,
			"agent_id": c.AgentID,
			"user_id":  c.UserID,
		},
		Limit: c.Limit,
	})
	if err != nil {
		a.logger.Warn("edge search failed", "error", err)
		return
	}

	for _, doc := range docs {
		e := a.DocToEdge(doc)
		if score, ok := doc["_score"].(float64); ok {
			e.Score = score
		}

		c.Edges = append(c.Edges, *e)
	}
}

// searchEntities 向量检索 Entities（从 OpenSearch 锚定）
func (a *RetrievalAction) searchEntities(c *domain.RecallContext) {
	if a.vectorStore == nil {
		return
	}

	// 使用向量检索锚定实体（Grounding）
	docs, err := a.vectorStore.Search(c.Context, storage.SearchQuery{
		Embedding: c.Embedding,
		Filters: map[string]any{
			"type":     domain.DocTypeEntity,
			"agent_id": c.AgentID,
			"user_id":  c.UserID,
		},
		Limit: c.Limit,
	})
	if err != nil {
		a.logger.Warn("entity vector search failed", "error", err)
		return
	}

	for _, doc := range docs {
		entity := a.DocToEntity(doc)
		if score, ok := doc["_score"].(float64); ok {
			entity.Score = score
		}

		c.Entities = append(c.Entities, *entity)
	}
}

// expandByGraphTraversal 通过图遍历扩展结果
func (a *RetrievalAction) expandByGraphTraversal(c *domain.RecallContext) {
	if a.graphStore == nil {
		return
	}

	seenEntities := make(map[string]bool)
	for _, e := range c.Entities {
		seenEntities[e.ID] = true
	}

	for _, startEntity := range c.Entities {
		nodes, err := a.graphStore.Traverse(
			c.Context,
			LabelEntity, "id", startEntity.ID,
			nil,
			"both",
			c.Options.MaxHops,
			c.Limit,
		)
		if err != nil {
			a.logger.Warn("graph traversal failed", "entity_id", startEntity.ID, "error", err)
			continue
		}

		for _, props := range nodes {
			entityID := getString(props, "id")
			if seenEntities[entityID] {
				continue
			}
			seenEntities[entityID] = true

			entity := domain.Entity{
				ID:          entityID,
				AgentID:     getString(props, "agent_id"),
				UserID:      getString(props, "user_id"),
				Name:        getString(props, "name"),
				Type:        domain.EntityType(getString(props, "type")),
				Description: getString(props, "description"),
			}
			c.Entities = append(c.Entities, entity)
		}
	}
}

// FormatMemoryContext 将检索结果格式化为 LLM prompt
func FormatMemoryContext(c *domain.RecallContext) string {
	var parts []string

	// 对话摘要
	if len(c.Summaries) > 0 {
		parts = append(parts, "## 对话摘要")
		for _, s := range c.Summaries {
			if s.Topic != "" {
				parts = append(parts, fmt.Sprintf("- [%s] %s", s.Topic, s.Content))
			} else {
				parts = append(parts, fmt.Sprintf("- %s", s.Content))
			}
		}
	}

	// 用户信息（Edges）
	if len(c.Edges) > 0 {
		parts = append(parts, "\n## 用户信息")
		for _, e := range c.Edges {
			parts = append(parts, fmt.Sprintf("- %s", e.Fact))
		}
	}

	// 相关对话
	if len(c.Episodes) > 0 {
		parts = append(parts, "\n## 相关对话记录")
		for _, ep := range c.Episodes {
			name := ep.Name
			if name == "" {
				name = ep.Role
			}
			parts = append(parts, fmt.Sprintf("- [%s] %s", name, ep.Content))
		}
	}

	// 相关实体
	if len(c.Entities) > 0 {
		parts = append(parts, "\n## 提及的实体")
		for _, e := range c.Entities {
			if e.Description != "" {
				parts = append(parts, fmt.Sprintf("- %s: %s", e.Name, e.Description))
			} else {
				parts = append(parts, fmt.Sprintf("- %s (%s)", e.Name, e.Type))
			}
		}
	}

	if len(parts) == 0 {
		return "没有找到相关的记忆信息。"
	}

	return strings.Join(parts, "\n")
}

// ============================================================================
// 截断函数（按数量和预算限制）
// ============================================================================

// truncateSummaries 截断 Summaries 到指定数量
func (a *RetrievalAction) truncateSummaries(c *domain.RecallContext, maxCount int) {
	if len(c.Summaries) > maxCount {
		c.Summaries = c.Summaries[:maxCount]
	}
}

// truncateEdges 截断 Edges（考虑数量和 token 预算）
func (a *RetrievalAction) truncateEdges(c *domain.RecallContext, maxCount, remainingTokens int) {
	if len(c.Edges) > maxCount {
		c.Edges = c.Edges[:maxCount]
	}
	// 按 token 预算进一步截断
	c.Edges = truncateByTokens(c.Edges, remainingTokens, func(e domain.Edge) int {
		return estimateTokens(e.Fact)
	})
}

// truncateEntities 截断 Entities（考虑数量和 token 预算）
func (a *RetrievalAction) truncateEntities(c *domain.RecallContext, maxCount, remainingTokens int) {
	if len(c.Entities) > maxCount {
		c.Entities = c.Entities[:maxCount]
	}
	// 按 token 预算进一步截断
	c.Entities = truncateByTokens(c.Entities, remainingTokens, func(e domain.Entity) int {
		return estimateTokens(e.Name + e.Description)
	})
}

// truncateEpisodes 截断 Episodes（考虑数量和 token 预算）
func (a *RetrievalAction) truncateEpisodes(c *domain.RecallContext, maxCount, remainingTokens int) {
	if len(c.Episodes) > maxCount {
		c.Episodes = c.Episodes[:maxCount]
	}
	// 按 token 预算进一步截断
	c.Episodes = truncateByTokens(c.Episodes, remainingTokens, func(e domain.Episode) int {
		return estimateTokens(e.Content)
	})
}

// filterCoveredEpisodes 过滤已被 Summary 覆盖的 Episodes
func (a *RetrievalAction) filterCoveredEpisodes(c *domain.RecallContext) {
	if len(c.Summaries) == 0 || len(c.Episodes) == 0 {
		return
	}

	// 收集 Summary 覆盖的 Episode IDs
	coveredIDs := make(map[string]bool)
	for _, s := range c.Summaries {
		for _, id := range s.EpisodeIDs {
			coveredIDs[id] = true
		}
	}

	// 过滤未被覆盖的 Episodes
	filtered := make([]domain.Episode, 0, len(c.Episodes))
	for _, ep := range c.Episodes {
		if !coveredIDs[ep.ID] {
			filtered = append(filtered, ep)
		}
	}
	c.Episodes = filtered
}

// ============================================================================
// Token 估算函数
// ============================================================================

// estimateTokens 估算文本的 token 数量
func estimateTokens(text string) int {
	charCount := utf8.RuneCountInString(text)
	return int(float64(charCount) / CharsPerToken)
}

// estimateSummaryTokens 估算 Summaries 的总 token 数
func (a *RetrievalAction) estimateSummaryTokens(summaries []domain.Summary) int {
	total := 0
	for _, s := range summaries {
		total += estimateTokens(s.Topic + s.Content)
	}
	return total
}

// estimateEdgeTokens 估算 Edges 的总 token 数
func (a *RetrievalAction) estimateEdgeTokens(edges []domain.Edge) int {
	total := 0
	for _, e := range edges {
		total += estimateTokens(e.Fact)
	}
	return total
}

// estimateEntityTokens 估算 Entities 的总 token 数
func (a *RetrievalAction) estimateEntityTokens(entities []domain.Entity) int {
	total := 0
	for _, e := range entities {
		total += estimateTokens(e.Name + e.Description)
	}
	return total
}

// estimateEpisodeTokens 估算 Episodes 的总 token 数
func (a *RetrievalAction) estimateEpisodeTokens(episodes []domain.Episode) int {
	total := 0
	for _, ep := range episodes {
		total += estimateTokens(ep.Content)
	}
	return total
}

// truncateByTokens 通用的按 token 预算截断函数
func truncateByTokens[T any](items []T, maxTokens int, estimator func(T) int) []T {
	if maxTokens <= 0 {
		return nil
	}

	var result []T
	usedTokens := 0

	for _, item := range items {
		tokens := estimator(item)
		if usedTokens+tokens > maxTokens {
			break
		}
		result = append(result, item)
		usedTokens += tokens
	}

	return result
}

// ============================================================================
// Helper functions
// ============================================================================

func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
