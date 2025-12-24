package mcp

// Tool represents an MCP tool definition
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

// InputSchema defines the JSON schema for tool input
type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

// Property defines a property in the schema
type Property struct {
	Type        string              `json:"type"`
	Description string              `json:"description,omitempty"`
	Enum        []string            `json:"enum,omitempty"`
	Items       *Property           `json:"items,omitempty"`
	Properties  map[string]Property `json:"properties,omitempty"`
	Default     any                 `json:"default,omitempty"`
}

// MemoryTools defines all available MCP tools for memory operations
var MemoryTools = []Tool{
	{
		Name:        "memory_generate",
		Description: "从对话中提取并存储记忆。会自动分类为工作记忆、情景记忆或语义记忆。",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"agent_id": {
					Type:        "string",
					Description: "AI 角色标识",
				},
				"user_id": {
					Type:        "string",
					Description: "用户标识",
				},
				"session_id": {
					Type:        "string",
					Description: "会话标识",
				},
				"conversation": {
					Type:        "array",
					Description: "对话消息列表",
					Items: &Property{
						Type: "object",
						Properties: map[string]Property{
							"role":    {Type: "string", Description: "角色: user/assistant/system"},
							"content": {Type: "string", Description: "消息内容"},
							"name":    {Type: "string", Description: "发送者名称"},
						},
					},
				},
				"session_date": {
					Type:        "string",
					Description: "会话日期 (YYYY-MM-DD)",
				},
			},
			Required: []string{"agent_id", "user_id", "session_id", "conversation"},
		},
	},
	{
		Name:        "memory_create",
		Description: "直接创建一条记忆，跳过自动提取流程。",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"agent_id": {
					Type:        "string",
					Description: "AI 角色标识",
				},
				"user_id": {
					Type:        "string",
					Description: "用户标识",
				},
				"session_id": {
					Type:        "string",
					Description: "会话标识（工作记忆和情景记忆必填）",
				},
				"type": {
					Type:        "string",
					Description: "记忆类型",
					Enum:        []string{"working", "episodic", "semantic"},
				},
				"content": {
					Type:        "string",
					Description: "记忆内容",
				},
				"importance": {
					Type:        "number",
					Description: "重要性 (0.0-1.0)",
					Default:     0.5,
				},
				"ttl_minutes": {
					Type:        "integer",
					Description: "工作记忆的存活时间（分钟）",
				},
				"metadata": {
					Type:        "object",
					Description: "额外元数据",
				},
			},
			Required: []string{"agent_id", "user_id", "type", "content"},
		},
	},
	{
		Name:        "memory_retrieve",
		Description: "检索相关记忆。会从工作记忆、情景记忆、语义记忆三个层次召回并合并结果。",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"agent_id": {
					Type:        "string",
					Description: "AI 角色标识",
				},
				"user_id": {
					Type:        "string",
					Description: "用户标识",
				},
				"session_id": {
					Type:        "string",
					Description: "当前会话标识（用于工作记忆召回）",
				},
				"query": {
					Type:        "string",
					Description: "查询内容",
				},
				"memory_types": {
					Type:        "array",
					Description: "要检索的记忆类型（默认全部）",
					Items:       &Property{Type: "string", Enum: []string{"working", "episodic", "semantic"}},
				},
				"limit": {
					Type:        "integer",
					Description: "每种类型返回的最大数量",
					Default:     5,
				},
				"min_importance": {
					Type:        "number",
					Description: "最小重要性阈值",
				},
				"time_range_days": {
					Type:        "integer",
					Description: "时间范围（天）",
				},
			},
			Required: []string{"agent_id", "user_id", "session_id", "query"},
		},
	},
	{
		Name:        "memory_forget",
		Description: "根据策略遗忘记忆。支持按重要性、时间、容量等策略。",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"agent_id": {
					Type:        "string",
					Description: "AI 角色标识",
				},
				"user_id": {
					Type:        "string",
					Description: "用户标识",
				},
				"strategy": {
					Type:        "string",
					Description: "遗忘策略",
					Enum:        []string{"importance", "time", "capacity", "expired"},
				},
				"importance_threshold": {
					Type:        "number",
					Description: "重要性阈值（importance 策略）",
					Default:     0.2,
				},
				"max_age_days": {
					Type:        "integer",
					Description: "最大存活天数（time 策略）",
					Default:     30,
				},
				"max_capacity": {
					Type:        "integer",
					Description: "最大容量（capacity 策略）",
					Default:     1000,
				},
				"memory_types": {
					Type:        "array",
					Description: "要处理的记忆类型",
					Items:       &Property{Type: "string", Enum: []string{"working", "episodic", "semantic"}},
				},
			},
			Required: []string{"agent_id", "user_id", "strategy"},
		},
	},
	{
		Name:        "memory_consolidate",
		Description: "整合记忆。将重要的工作记忆提升为情景记忆，从情景记忆中提取语义记忆。",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"agent_id": {
					Type:        "string",
					Description: "AI 角色标识",
				},
				"user_id": {
					Type:        "string",
					Description: "用户标识",
				},
				"session_id": {
					Type:        "string",
					Description: "要整合的会话（留空则整合所有）",
				},
				"importance_threshold": {
					Type:        "number",
					Description: "工作记忆提升阈值",
					Default:     0.7,
				},
				"working_to_episodic": {
					Type:        "boolean",
					Description: "是否执行 工作记忆 → 情景记忆",
					Default:     true,
				},
				"episodic_to_semantic": {
					Type:        "boolean",
					Description: "是否执行 情景记忆 → 语义记忆提取",
					Default:     true,
				},
			},
			Required: []string{"agent_id", "user_id"},
		},
	},
	{
		Name:        "memory_delete",
		Description: "删除指定的记忆。",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"memory_id": {
					Type:        "string",
					Description: "要删除的记忆 ID",
				},
			},
			Required: []string{"memory_id"},
		},
	},
}
