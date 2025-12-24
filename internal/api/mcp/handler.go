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
	case "memory_forget":
		return h.handleForget(ctx, req.Arguments)
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
		"成功添加记忆:\n- 摘要记忆: %d\n- 事件三元组: %d\n- 事件关系: %d",
		len(resp.Summaries),
		len(resp.Events),
		len(resp.EventRelations),
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

// handleForget handles memory_forget tool call
func (h *Handler) handleForget(ctx context.Context, args json.RawMessage) ToolCallResponse {
	var req domain.ForgetRequest
	if err := json.Unmarshal(args, &req); err != nil {
		return errorResponse(fmt.Sprintf("invalid arguments: %v", err))
	}

	if req.AgentID == "" || req.UserID == "" {
		return errorResponse("agent_id and user_id are required")
	}

	resp, err := h.memory.Forget(ctx, &req)
	if err != nil {
		return errorResponse(fmt.Sprintf("forget failed: %v", err))
	}

	return successResponse(fmt.Sprintf(
		"遗忘完成:\n- 工作记忆遗忘: %d\n- 事件遗忘: %d\n- 事实过期: %d",
		resp.WorkingForgot,
		resp.EventsForgot,
		resp.FactsExpired,
	))
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

	if len(resp.Facts) > 0 {
		parts = append(parts, "## 用户事实")
		for _, f := range resp.Facts {
			ts := f.CreatedAt.Format("2006-01-02")
			parts = append(parts, fmt.Sprintf("- [%s] %s", ts, f.Content))
		}
	}

	if len(resp.WorkingMem) > 0 {
		parts = append(parts, "\n## 工作记忆")
		for _, w := range resp.WorkingMem {
			ts := w.CreatedAt.Format("2006-01-02")
			parts = append(parts, fmt.Sprintf("- [%s] %s", ts, w.Content))
		}
	}

	if len(resp.Events) > 0 {
		parts = append(parts, "\n## 相关事件")
		for _, e := range resp.Events {
			ts := e.CreatedAt.Format("2006-01-02")
			parts = append(parts, fmt.Sprintf("- [%s] %s %s %s", ts, e.Argument1, e.TriggerWord, e.Argument2))
		}
	}

	if len(resp.ShortTerm) > 0 {
		parts = append(parts, "\n## 近期对话")
		for _, msg := range resp.ShortTerm {
			name := msg.Name
			if name == "" {
				name = msg.Role
			}
			parts = append(parts, fmt.Sprintf("- [%s] %s", name, truncate(msg.Content, 100)))
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
