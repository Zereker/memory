package domain

import "context"

// ============================================================================
// Action Interfaces - 处理链
// ============================================================================

// AddAction 用于 Add 流程的 action
type AddAction interface {
	Name() string
	Handle(*AddContext)
}

// RecallAction 用于 Recall 流程的 action
type RecallAction interface {
	Name() string
	HandleRecall(*RecallContext)
}

// ============================================================================
// Context Types
// ============================================================================

// TokenUsage 记录 token 使用量
type TokenUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// baseContext 包含链式处理的通用字段
type baseContext struct {
	context.Context

	// Scope 标识
	AgentID   string
	UserID    string
	SessionID string

	// 元数据
	Metadata map[string]any

	// Token 使用量统计
	TokenUsages map[string]TokenUsage

	// 链式控制
	index   int
	aborted bool
	err     error
}

// Set 存储元数据
func (c *baseContext) Set(key string, value any) {
	c.Metadata[key] = value
}

// Get 获取元数据
func (c *baseContext) Get(key string) (any, bool) {
	val, ok := c.Metadata[key]
	return val, ok
}

// Abort 终止链式执行
func (c *baseContext) Abort() {
	c.aborted = true
}

// IsAborted 返回链是否被终止
func (c *baseContext) IsAborted() bool {
	return c.aborted
}

// SetError 设置错误并终止链
func (c *baseContext) SetError(err error) {
	c.err = err
	c.aborted = true
}

// Error 返回错误
func (c *baseContext) Error() error {
	return c.err
}

// AddTokenUsage 记录 token 使用量
func (c *baseContext) AddTokenUsage(actionName string, inputTokens, outputTokens int) {
	if c.TokenUsages == nil {
		c.TokenUsages = make(map[string]TokenUsage)
	}

	existing := c.TokenUsages[actionName]
	existing.InputTokens += inputTokens
	existing.OutputTokens += outputTokens
	c.TokenUsages[actionName] = existing
}

// GetTokenUsage 获取指定 action 的 token 使用量
func (c *baseContext) GetTokenUsage(actionName string) TokenUsage {
	return c.TokenUsages[actionName]
}

// TotalTokenUsage 获取所有 action 的 token 使用量总和
func (c *baseContext) TotalTokenUsage() TokenUsage {
	var total TokenUsage

	for _, usage := range c.TokenUsages {
		total.InputTokens += usage.InputTokens
		total.OutputTokens += usage.OutputTokens
	}

	return total
}

// ============================================================================
// AddContext - 用于记忆写入流程
// ============================================================================

// AddContext 记忆写入上下文
type AddContext struct {
	baseContext

	// 输入
	Messages Messages

	// 输出 - 三层认知模型
	ShortTermWindow *ShortTermMemory // Layer 1: 短期记忆窗口
	Summaries       []SummaryMemory  // Layer 2: 摘要记忆
	Events          []EventTriplet   // Layer 3: 事件三元组
	EventRelations  []EventRelation  // Layer 3: 事件关系

	// 配置
	Language string // 语言设置

	// 链式处理器
	actions []AddAction
}

// NewAddContext 创建新的 AddContext
func NewAddContext(ctx context.Context, agentID, userID, sessionID string) *AddContext {
	return &AddContext{
		baseContext: baseContext{
			Context:     ctx,
			AgentID:     agentID,
			UserID:      userID,
			SessionID:   sessionID,
			Metadata:    make(map[string]any),
			TokenUsages: make(map[string]TokenUsage),
		},
		Language: "zh_CN",
	}
}

// Next 调用链中的下一个 action
func (c *AddContext) Next() {
	c.index++
	for c.index < len(c.actions) {
		if c.aborted {
			return
		}

		c.actions[c.index].Handle(c)
		c.index++
	}
}

// AddSummaries 添加摘要记忆
func (c *AddContext) AddSummaries(summaries ...SummaryMemory) {
	c.Summaries = append(c.Summaries, summaries...)
}

// AddEvents 添加事件三元组
func (c *AddContext) AddEvents(events ...EventTriplet) {
	c.Events = append(c.Events, events...)
}

// AddEventRelations 添加事件关系
func (c *AddContext) AddEventRelations(relations ...EventRelation) {
	c.EventRelations = append(c.EventRelations, relations...)
}

// LanguageName 返回语言名称
func (c *AddContext) LanguageName() string {
	switch c.Language {
	case "en_US":
		return "English"
	case "ja_JP":
		return "日本語"
	default:
		return "中文"
	}
}

// ============================================================================
// RecallContext - 用于记忆检索流程
// ============================================================================

// RecallContext 记忆检索上下文
type RecallContext struct {
	baseContext

	// 查询参数
	Query     string
	Embedding []float32
	Limit     int
	Options   RetrieveOptions

	// 检索结果 - 三层认知结构
	Facts      []SummaryMemory // fact 类型摘要
	WorkingMem []SummaryMemory // working 类型摘要
	Events     []EventTriplet  // 事件三元组
	ShortTerm  Messages        // 短期记忆窗口

	// 链式处理器
	actions []RecallAction
}

// NewRecallContext 创建新的 RecallContext
func NewRecallContext(ctx context.Context, req *RetrieveRequest) *RecallContext {
	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}

	return &RecallContext{
		baseContext: baseContext{
			Context:     ctx,
			AgentID:     req.AgentID,
			UserID:      req.UserID,
			SessionID:   req.SessionID,
			Metadata:    make(map[string]any),
			TokenUsages: make(map[string]TokenUsage),
		},
		Query:   req.Query,
		Limit:   limit,
		Options: req.Options,
	}
}

// Next 调用链中的下一个 action
func (c *RecallContext) Next() {
	c.index++
	for c.index < len(c.actions) {
		if c.aborted {
			return
		}
		c.actions[c.index].HandleRecall(c)
		c.index++
	}
}

// TotalResults 返回检索结果总数
func (c *RecallContext) TotalResults() int {
	return len(c.Facts) + len(c.WorkingMem) + len(c.Events) + len(c.ShortTerm)
}

// ============================================================================
// Action Chains
// ============================================================================

// ActionChain 管理 AddAction 处理器链
type ActionChain struct {
	actions []AddAction
}

// NewActionChain 创建新的 Action 链
func NewActionChain() *ActionChain {
	return &ActionChain{
		actions: []AddAction{},
	}
}

// Use 添加 action 到链
func (chain *ActionChain) Use(actions ...AddAction) *ActionChain {
	chain.actions = append(chain.actions, actions...)
	return chain
}

// Run 顺序执行链中的所有 action
func (chain *ActionChain) Run(c *AddContext) {
	c.actions = chain.actions
	c.index = -1
	c.Next()
}

// RecallChain 管理 RecallAction 处理器链
type RecallChain struct {
	actions []RecallAction
}

// NewRecallChain 创建新的 Recall 链
func NewRecallChain() *RecallChain {
	return &RecallChain{
		actions: []RecallAction{},
	}
}

// Use 添加 action 到链
func (chain *RecallChain) Use(actions ...RecallAction) *RecallChain {
	chain.actions = append(chain.actions, actions...)
	return chain
}

// Run 顺序执行链中的所有 action
func (chain *RecallChain) Run(c *RecallContext) {
	c.actions = chain.actions
	c.index = -1
	c.Next()
}
