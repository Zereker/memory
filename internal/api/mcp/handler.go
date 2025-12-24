package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Zereker/memory/internal/action"
	"github.com/Zereker/memory/internal/domain"
)

// Handler handles MCP tool calls
type Handler struct {
	memory *action.Memory
}

// NewHandler creates a new MCP handler
func NewHandler(memory *action.Memory) *Handler {
	return &Handler{
		memory: memory,
	}
}

// ToolCallRequest represents an MCP tool call request
type ToolCallRequest struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolCallResponse represents an MCP tool call response
type ToolCallResponse struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock represents a content block in the response
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// HandleToolCall handles an MCP tool call
func (h *Handler) HandleToolCall(ctx context.Context, req ToolCallRequest) ToolCallResponse {
	switch req.Name {
	case "memory_add":
		return h.handleAdd(ctx, req.Arguments)
	case "memory_retrieve":
		return h.handleRetrieve(ctx, req.Arguments)
	case "memory_delete":
		return h.handleDelete(ctx, req.Arguments)
	default:
		return errorResponse(fmt.Sprintf("unknown tool: %s", req.Name))
	}
}

// handleAdd handles memory_add tool call
func (h *Handler) handleAdd(ctx context.Context, args json.RawMessage) ToolCallResponse {
	var req domain.AddRequest
	if err := json.Unmarshal(args, &req); err != nil {
		return errorResponse(fmt.Sprintf("invalid arguments: %v", err))
	}

	resp, err := h.memory.Add(ctx, &req)
	if err != nil {
		return errorResponse(fmt.Sprintf("add failed: %v", err))
	}

	return successResponse(fmt.Sprintf(
		"成功添加记忆:\n- Episodes: %d\n- Entities: %d\n- Edges: %d",
		len(resp.Episodes),
		len(resp.Entities),
		len(resp.Edges),
	))
}

// handleRetrieve handles memory_retrieve tool call
func (h *Handler) handleRetrieve(ctx context.Context, args json.RawMessage) ToolCallResponse {
	var req domain.RetrieveRequest
	if err := json.Unmarshal(args, &req); err != nil {
		return errorResponse(fmt.Sprintf("invalid arguments: %v", err))
	}

	resp, err := h.memory.Retrieve(ctx, &req)
	if err != nil {
		return errorResponse(fmt.Sprintf("retrieve failed: %v", err))
	}

	// 返回格式化的记忆上下文
	if resp.MemoryContext != "" {
		return successResponse(resp.MemoryContext)
	}

	return successResponse(formatRetrieveResponse(resp))
}

// handleDelete handles memory_delete tool call
func (h *Handler) handleDelete(ctx context.Context, args json.RawMessage) ToolCallResponse {
	var req struct {
		MemoryID string `json:"memory_id"`
	}
	if err := json.Unmarshal(args, &req); err != nil {
		return errorResponse(fmt.Sprintf("invalid arguments: %v", err))
	}

	if err := h.memory.Delete(ctx, req.MemoryID); err != nil {
		return errorResponse(fmt.Sprintf("delete failed: %v", err))
	}

	return successResponse(fmt.Sprintf("成功删除记忆: %s", req.MemoryID))
}

// formatRetrieveResponse 格式化检索响应
func formatRetrieveResponse(resp *domain.RetrieveResponse) string {
	var parts []string

	if len(resp.Edges) > 0 {
		parts = append(parts, "## 用户信息")
		for _, edge := range resp.Edges {
			parts = append(parts, fmt.Sprintf("- %s", edge.Fact))
		}
	}

	if len(resp.Episodes) > 0 {
		parts = append(parts, "\n## 相关对话")
		for _, ep := range resp.Episodes {
			name := ep.Name
			if name == "" {
				name = ep.Role
			}
			parts = append(parts, fmt.Sprintf("- [%s] %s", name, truncate(ep.Content, 100)))
		}
	}

	if len(resp.Entities) > 0 {
		parts = append(parts, "\n## 相关实体")
		for _, entity := range resp.Entities {
			if entity.Description != "" {
				parts = append(parts, fmt.Sprintf("- %s: %s", entity.Name, entity.Description))
			} else {
				parts = append(parts, fmt.Sprintf("- %s (%s)", entity.Name, entity.Type))
			}
		}
	}

	if len(parts) == 0 {
		return "没有找到相关的记忆信息。"
	}

	return strings.Join(parts, "\n")
}

// Helper functions

func successResponse(text string) ToolCallResponse {
	return ToolCallResponse{
		Content: []ContentBlock{
			{Type: "text", Text: text},
		},
	}
}

func errorResponse(text string) ToolCallResponse {
	return ToolCallResponse{
		Content: []ContentBlock{
			{Type: "text", Text: text},
		},
		IsError: true,
	}
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
