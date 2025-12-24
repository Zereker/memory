package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/Zereker/memory/internal/action"
	"github.com/Zereker/memory/pkg/log"
)

// Server represents an MCP server
type Server struct {
	logger  *slog.Logger
	handler *Handler
	name    string
	version string
}

// ServerConfig contains server configuration
type ServerConfig struct {
	Name    string
	Version string
}

// NewServer creates a new MCP server
func NewServer(memory *action.Memory, config ServerConfig) *Server {
	return &Server{
		logger:  log.Logger("mcp"),
		handler: NewHandler(memory),
		name:    config.Name,
		version: config.Version,
	}
}

// JSON-RPC types
type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Result  any    `json:"result,omitempty"`
	Error   *Error `json:"error,omitempty"`
}

// Error represents a JSON-RPC error
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// MCP message types
type initializeParams struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities"`
	ClientInfo      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"clientInfo"`
}

type initializeResult struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities"`
	ServerInfo      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"serverInfo"`
}

type toolsListResult struct {
	Tools []Tool `json:"tools"`
}

type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// RunStdio runs the MCP server using stdio transport
func (s *Server) RunStdio(ctx context.Context) error {
	s.logger.Info("starting stdio server", "name", s.name, "version", s.version)

	reader := bufio.NewReader(os.Stdin)
	writer := os.Stdout

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Read JSON-RPC message
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				s.logger.Info("stdin closed")
				return nil
			}
			return fmt.Errorf("read error: %w", err)
		}

		// Parse request
		var req jsonRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			s.writeError(writer, nil, -32700, "Parse error", err.Error())
			continue
		}

		// Handle request
		resp := s.handleRequest(ctx, &req)

		// Write response
		if err := s.writeResponse(writer, resp); err != nil {
			s.logger.Error("write error", "error", err)
		}
	}
}

// handleRequest handles a JSON-RPC request
func (s *Server) handleRequest(ctx context.Context, req *jsonRPCRequest) *jsonRPCResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "initialized":
		return s.handleInitialized(req)
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(ctx, req)
	case "ping":
		return s.handlePing(req)
	default:
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &Error{
				Code:    -32601,
				Message: "Method not found",
				Data:    req.Method,
			},
		}
	}
}

// handleInitialize handles the initialize request
func (s *Server) handleInitialize(req *jsonRPCRequest) *jsonRPCResponse {
	var params initializeParams
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	s.logger.Info("initialize",
		"client", params.ClientInfo.Name,
		"clientVersion", params.ClientInfo.Version,
		"protocol", params.ProtocolVersion,
	)

	result := initializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: map[string]any{
			"tools": map[string]any{},
		},
	}
	result.ServerInfo.Name = s.name
	result.ServerInfo.Version = s.version

	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

// handleInitialized handles the initialized notification
func (s *Server) handleInitialized(req *jsonRPCRequest) *jsonRPCResponse {
	s.logger.Info("initialized")
	// This is a notification, no response needed
	return nil
}

// handleToolsList handles the tools/list request
func (s *Server) handleToolsList(req *jsonRPCRequest) *jsonRPCResponse {
	s.logger.Debug("tools/list")

	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  toolsListResult{Tools: MemoryTools},
	}
}

// handleToolsCall handles the tools/call request
func (s *Server) handleToolsCall(ctx context.Context, req *jsonRPCRequest) *jsonRPCResponse {
	var params toolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &Error{
				Code:    -32602,
				Message: "Invalid params",
				Data:    err.Error(),
			},
		}
	}

	s.logger.Info("tools/call", "tool", params.Name)

	toolReq := ToolCallRequest{
		Name:      params.Name,
		Arguments: params.Arguments,
	}

	result := s.handler.HandleToolCall(ctx, toolReq)

	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

// handlePing handles the ping request
func (s *Server) handlePing(req *jsonRPCRequest) *jsonRPCResponse {
	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]any{},
	}
}

// writeResponse writes a JSON-RPC response
func (s *Server) writeResponse(w io.Writer, resp *jsonRPCResponse) error {
	if resp == nil {
		return nil
	}

	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(w, "%s\n", data)
	return err
}

// writeError writes a JSON-RPC error response
func (s *Server) writeError(w io.Writer, id any, code int, message, data string) error {
	resp := &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &Error{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	return s.writeResponse(w, resp)
}
