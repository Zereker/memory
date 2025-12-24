package domain

import (
	"time"
)

// ============================================================================
// 文档类型常量
// ============================================================================

const (
	DocTypeSummary = "summary" // 摘要记忆（Layer 2）
	DocTypeEvent   = "event"   // 事件三元组（Layer 3）
)

// ============================================================================
// 记忆类型常量
// ============================================================================

const (
	MemoryTypeFact    = "fact"    // 事实记忆（长期稳定）
	MemoryTypeWorking = "working" // 工作记忆（短期会话相关）
)

// ============================================================================
// 角色常量
// ============================================================================

const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleSystem    = "system"
)

// ============================================================================
// 关系类型常量
// ============================================================================

const (
	RelationCausal   = "causal"   // 因果关系
	RelationTemporal = "temporal" // 时序关系
)

// ============================================================================
// 三层认知记忆模型
// Layer 1: ShortTermMemory（短期记忆 - 感知层）
// Layer 2: SummaryMemory（摘要记忆 - 经验层）
// Layer 3: EventTriplet + EventRelation（事件图谱 - 推理层）
// ============================================================================

// ============================================================================
// Layer 1: ShortTermMemory - 短期记忆（内存滑动窗口）
// ============================================================================

// ShortTermMemory 短期记忆滑动窗口
type ShortTermMemory struct {
	AgentID   string    `json:"agent_id"`
	UserID    string    `json:"user_id"`
	SessionID string    `json:"session_id"`
	Messages  Messages  `json:"messages"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ============================================================================
// Layer 2: SummaryMemory - 摘要记忆
// ============================================================================

// SummaryMemory 摘要记忆（带重要性打分 + fact/working 分类）
type SummaryMemory struct {
	ID      string `json:"id"`
	AgentID string `json:"agent_id"`
	UserID  string `json:"user_id"`

	// 内容
	Content    string   `json:"content"`     // 摘要内容
	MemoryType string   `json:"memory_type"` // fact / working
	Importance float64  `json:"importance"`  // 重要性 0-1
	Keywords   []string `json:"keywords"`    // 关键词列表

	// 向量
	Embedding []float32 `json:"embedding,omitempty"`

	// 访问统计
	AccessCount    int       `json:"access_count"`
	LastAccessedAt time.Time `json:"last_accessed_at"`

	// 保护标记
	IsProtected bool `json:"is_protected"` // importance >= 0.9 自动标记

	// 时间
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	ExpiredAt *time.Time `json:"expired_at,omitempty"` // 软删除/冲突过期

	// 检索分数 (查询时填充)
	Score float64 `json:"score,omitempty"`
}

// ============================================================================
// Layer 3: EventTriplet - 事件三元组
// ============================================================================

// EventTriplet 事件三元组
type EventTriplet struct {
	ID      string `json:"id"`
	AgentID string `json:"agent_id"`
	UserID  string `json:"user_id"`

	// 三元组
	TriggerWord string `json:"trigger_word"` // 触发词（谓词）
	Argument1   string `json:"argument1"`    // 论元1（主语/施事）
	Argument2   string `json:"argument2"`    // 论元2（宾语/受事）

	// 向量
	TriggerEmbedding []float32 `json:"trigger_embedding,omitempty"` // 触发词向量

	// 访问统计
	AccessCount    int       `json:"access_count"`
	LastAccessedAt time.Time `json:"last_accessed_at"`

	// 时间
	CreatedAt time.Time `json:"created_at"`

	// 检索分数 (查询时填充)
	Score float64 `json:"score,omitempty"`
}

// ============================================================================
// Layer 3: EventRelation - 事件关系
// ============================================================================

// EventRelation 事件间的因果/时序关系
type EventRelation struct {
	ID           string `json:"id"`
	RelationType string `json:"relation_type"` // causal / temporal
	FromEventID  string `json:"from_event_id"`
	ToEventID    string `json:"to_event_id"`
	CreatedAt    time.Time `json:"created_at"`
}

// ============================================================================
// Message 对话消息
// ============================================================================

// Message 表示一条对话消息
type Message struct {
	Role    string `json:"role"`           // user / assistant / system
	Content string `json:"content"`        // 消息内容
	Name    string `json:"name,omitempty"` // 发言者名称
}

// Messages 消息列表
type Messages []Message

// UserName 获取用户名称
func (m Messages) UserName() string {
	for _, msg := range m {
		if msg.Role == "user" && msg.Name != "" {
			return msg.Name
		}
	}
	return "user"
}

// AssistantName 获取助手名称
func (m Messages) AssistantName() string {
	for _, msg := range m {
		if msg.Role == "assistant" && msg.Name != "" {
			return msg.Name
		}
	}

	return "assistant"
}

// Format 格式化为对话文本
func (m Messages) Format() string {
	var result string

	for _, msg := range m {
		name := msg.Name
		if name == "" {
			name = msg.Role
		}

		result += name + ": " + msg.Content + "\n"
	}

	return result
}

// ============================================================================
// API Request/Response
// ============================================================================

// AddRequest 添加记忆请求
type AddRequest struct {
	AgentID   string    `json:"agent_id"`
	UserID    string    `json:"user_id"`
	SessionID string    `json:"session_id"`
	Messages  []Message `json:"messages"`
}

// AddResponse 添加记忆响应
type AddResponse struct {
	Success        bool            `json:"success"`
	Summaries      []SummaryMemory `json:"summaries,omitempty"`
	Events         []EventTriplet  `json:"events,omitempty"`
	EventRelations []EventRelation `json:"event_relations,omitempty"`
}

// RetrieveRequest 检索记忆请求
type RetrieveRequest struct {
	AgentID   string `json:"agent_id"`
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id,omitempty"`
	Query     string `json:"query"`
	Limit     int    `json:"limit,omitempty"`

	// 检索选项
	Options RetrieveOptions `json:"options,omitempty"`
}

// RetrieveOptions 检索选项
// 3-Bucket Token 预算分配策略
type RetrieveOptions struct {
	MaxTokens int `json:"max_tokens,omitempty"` // 总 token 预算，默认 2000

	// 各桶配额（-1 禁用，0 使用默认值，>0 自定义）
	MaxFacts   int `json:"max_facts,omitempty"`   // Fact 桶 token 预算
	MaxGraph   int `json:"max_graph,omitempty"`   // Graph 桶 token 预算
	MaxWorking int `json:"max_working,omitempty"` // Working 桶 token 预算
}

// RetrieveResponse 检索记忆响应
type RetrieveResponse struct {
	Success bool `json:"success"`

	// 三层结果
	Facts      []SummaryMemory `json:"facts,omitempty"`       // fact 类型摘要
	WorkingMem []SummaryMemory `json:"working_mem,omitempty"` // working 类型摘要
	Events     []EventTriplet  `json:"events,omitempty"`      // 事件三元组
	ShortTerm  Messages        `json:"short_term,omitempty"`  // 短期记忆窗口
	Total      int             `json:"total"`

	// 格式化后的记忆上下文 (用于 LLM prompt)
	MemoryContext string `json:"memory_context,omitempty"`
}

// ForgetRequest 遗忘记忆请求
type ForgetRequest struct {
	AgentID string `json:"agent_id"`
	UserID  string `json:"user_id"`
}

// ForgetResponse 遗忘记忆响应
type ForgetResponse struct {
	Success        bool `json:"success"`
	WorkingForgot  int  `json:"working_forgot"`
	EventsForgot   int  `json:"events_forgot"`
	FactsExpired   int  `json:"facts_expired"`
}
