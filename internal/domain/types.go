package domain

import (
	"time"
)

// ============================================================================
// 文档类型常量
// ============================================================================

const (
	DocTypeEpisode = "episode"
	DocTypeEntity  = "entity"
	DocTypeEdge    = "edge"
	DocTypeSummary = "summary"
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
// 三层子图模型
// Layer 1: Episode (原始对话)
// Layer 2: Entity + Edge (实体 + 关系)
// Layer 3: Summary (对话摘要)
// ============================================================================

// ============================================================================
// Layer 1: Episode - 原始对话存储
// ============================================================================

// Episode 表示一条原始对话记录
// 存储在 OpenSearch (向量检索)
type Episode struct {
	ID        string `json:"id"`
	AgentID   string `json:"agent_id"`
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`

	// 对话内容
	Role string `json:"role"` // user / assistant
	Name string `json:"name"` // 发言者名称

	// 主题
	Topic          string    `json:"topic"`
	TopicEmbedding []float32 `json:"topic_embedding"`

	// 内容
	Content   string    `json:"content"`
	Embedding []float32 `json:"content_embedding"`

	// 时间
	Timestamp time.Time `json:"timestamp"`  // 对话发生时间
	CreatedAt time.Time `json:"created_at"` // 入库时间

	// 检索分数 (查询时填充)
	Score float64 `json:"score,omitempty"`
}

// ============================================================================
// Layer 2: Entity - 实体节点
// ============================================================================

// EntityType 实体类型
type EntityType string

const (
	EntityTypePerson   EntityType = "person"   // 人物
	EntityTypePlace    EntityType = "place"    // 地点
	EntityTypeThing    EntityType = "thing"    // 物品/概念
	EntityTypeEvent    EntityType = "event"    // 事件
	EntityTypeEmotion  EntityType = "emotion"  // 情绪
	EntityTypeActivity EntityType = "activity" // 活动
)

// Entity 表示知识图谱中的实体节点
type Entity struct {
	ID          string     `json:"id"`
	AgentID     string     `json:"agent_id"`
	UserID      string     `json:"user_id"`
	Name        string     `json:"name"`        // 实体名称
	Type        EntityType `json:"type"`        // 实体类型
	Description string     `json:"description"` // 实体描述

	// 向量
	Embedding []float32 `json:"embedding,omitempty"`

	// 时间
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// 检索分数
	Score float64 `json:"score,omitempty"`
}

// ============================================================================
// Layer 2: Edge - 关系边
// ============================================================================

// Edge 表示实体间的关系
type Edge struct {
	ID       string `json:"id"`
	SourceID string `json:"source_id"` // 起始实体 ID
	TargetID string `json:"target_id"` // 目标实体 ID
	Relation string `json:"relation"`  // 关系类型 (如: 喜欢, 认识, 住在)
	Fact     string `json:"fact"`      // 事实描述 (如: "用户喜欢喝咖啡")

	// 向量
	Embedding []float32 `json:"embedding,omitempty"`

	// 双时间轴 (Bi-temporal)
	ValidAt   *time.Time `json:"valid_at,omitempty"`   // 事实生效时间
	InvalidAt *time.Time `json:"invalid_at,omitempty"` // 事实失效时间

	// 入库时间
	CreatedAt time.Time  `json:"created_at"`
	ExpiredAt *time.Time `json:"expired_at,omitempty"` // 记录过期时间

	// 溯源
	EpisodeIDs []string `json:"episode_ids,omitempty"` // 来源 Episode

	// 检索分数
	Score float64 `json:"score,omitempty"`
}

// IsValid 检查边在指定时间点是否有效
func (e *Edge) IsValid(at time.Time) bool {
	if e.ValidAt != nil && at.Before(*e.ValidAt) {
		return false
	}

	if e.InvalidAt != nil && at.After(*e.InvalidAt) {
		return false
	}

	return true
}

// ============================================================================
// Layer 3: Summary - 对话摘要
// ============================================================================

// Summary 表示一组对话的压缩摘要
type Summary struct {
	ID         string   `json:"id"`
	AgentID    string   `json:"agent_id"`
	UserID     string   `json:"user_id"`
	EpisodeIDs []string `json:"episode_ids"` // 关联的 Episode ID
	Content    string   `json:"content"`     // 摘要内容
	Topic      string   `json:"topic"`       // 主题标签

	// 向量
	Embedding []float32 `json:"embedding,omitempty"`

	// 时间
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// 检索分数
	Score float64 `json:"score,omitempty"`
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
	Success   bool      `json:"success"`
	Episodes  []Episode `json:"episodes,omitempty"`
	Entities  []Entity  `json:"entities,omitempty"`
	Edges     []Edge    `json:"edges,omitempty"`
	Summaries []Summary `json:"summaries,omitempty"`
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
// 按优先级检索：Summary > Edge > Entity > Episode
// 设置 Max* = 0 可禁用该类型检索，不设置则使用默认值
type RetrieveOptions struct {
	MaxHops   int `json:"max_hops,omitempty"`   // 图遍历最大跳数
	MaxTokens int `json:"max_tokens,omitempty"` // 总 token 预算，默认 2000

	// 各类型数量限制（-1 禁用，0 使用默认值，>0 自定义）
	MaxSummaries int `json:"max_summaries,omitempty"` // 默认 3
	MaxEdges     int `json:"max_edges,omitempty"`     // 默认 10
	MaxEntities  int `json:"max_entities,omitempty"`  // 默认 5
	MaxEpisodes  int `json:"max_episodes,omitempty"`  // 默认 5
}

// RetrieveResponse 检索记忆响应
type RetrieveResponse struct {
	Success   bool      `json:"success"`
	Episodes  []Episode `json:"episodes,omitempty"`
	Entities  []Entity  `json:"entities,omitempty"`
	Edges     []Edge    `json:"edges,omitempty"`
	Summaries []Summary `json:"summaries,omitempty"`
	Total     int       `json:"total"`

	// 格式化后的记忆上下文 (用于 LLM prompt)
	MemoryContext string `json:"memory_context,omitempty"`
}
