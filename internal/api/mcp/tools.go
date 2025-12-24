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
		Name:        "memory_add",
		Description: "从对话中提取并存储记忆。自动提取摘要记忆（事实/工作记忆）和事件三元组，并进行认知一致性检查。",
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
				"messages": {
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
			},
			Required: []string{"agent_id", "user_id", "session_id", "messages"},
		},
	},
	{
		Name:        "memory_retrieve",
		Description: "检索相关记忆。从事实记忆、工作记忆、事件图谱、短期记忆四个层次召回并合并结果，使用 3-Bucket Token 预算分配策略。",
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
					Description: "当前会话标识（用于短期记忆召回）",
				},
				"query": {
					Type:        "string",
					Description: "查询内容",
				},
				"limit": {
					Type:        "integer",
					Description: "每种类型返回的最大数量",
					Default:     5,
				},
				"max_tokens": {
					Type:        "integer",
					Description: "总 token 预算（默认 2000）",
					Default:     2000,
				},
			},
			Required: []string{"agent_id", "user_id", "query"},
		},
	},
	{
		Name:        "memory_forget",
		Description: "执行记忆遗忘。根据遗忘评分自动清理工作记忆和事件图谱，并过期超过 3 个月的事实记忆。",
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
