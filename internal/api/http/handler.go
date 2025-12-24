package http

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/Zereker/memory/internal/action"
	"github.com/Zereker/memory/internal/domain"
	"github.com/Zereker/memory/pkg/log"
)

// Handler handles HTTP API requests
type Handler struct {
	logger *slog.Logger
	memory *action.Memory
}

// NewHandler creates a new HTTP handler
func NewHandler(memory *action.Memory) *Handler {
	return &Handler{
		logger: log.Logger("http.handler"),
		memory: memory,
	}
}

// Response represents a standard API response
type Response struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

// RegisterRoutes registers all HTTP routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Memory operations
	mux.HandleFunc("POST /api/v1/memories/add", h.Add)
	mux.HandleFunc("POST /api/v1/memories/retrieve", h.Retrieve)
	mux.HandleFunc("GET /api/v1/memories/retrieve", h.Retrieve)
	mux.HandleFunc("POST /api/v1/memories/forget", h.Forget)
	mux.HandleFunc("DELETE /api/v1/memories/{id}", h.Delete)

	// Health check
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("GET /api/v1/health", h.Health)
}

// Add handles POST /api/v1/memories/add
func (h *Handler) Add(w http.ResponseWriter, r *http.Request) {
	var req domain.AddRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	resp, err := h.memory.Add(r.Context(), &req)
	if err != nil {
		h.logger.Error("add failed", "error", err)
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    resp,
	})
}

// Retrieve handles POST/GET /api/v1/memories/retrieve
func (h *Handler) Retrieve(w http.ResponseWriter, r *http.Request) {
	var req domain.RetrieveRequest

	if r.Method == http.MethodGet {
		req.AgentID = r.URL.Query().Get("agent_id")
		req.UserID = r.URL.Query().Get("user_id")
		req.SessionID = r.URL.Query().Get("session_id")
		req.Query = r.URL.Query().Get("query")
	} else {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
	}

	if req.AgentID == "" || req.UserID == "" || req.Query == "" {
		h.writeError(w, http.StatusBadRequest, "agent_id, user_id, and query are required")
		return
	}

	resp, err := h.memory.Retrieve(r.Context(), &req)
	if err != nil {
		h.logger.Error("retrieve failed", "error", err)
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    resp,
	})
}

// Forget handles POST /api/v1/memories/forget
func (h *Handler) Forget(w http.ResponseWriter, r *http.Request) {
	var req domain.ForgetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.AgentID == "" || req.UserID == "" {
		h.writeError(w, http.StatusBadRequest, "agent_id and user_id are required")
		return
	}

	resp, err := h.memory.Forget(r.Context(), &req)
	if err != nil {
		h.logger.Error("forget failed", "error", err)
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    resp,
	})
}

// Delete handles DELETE /api/v1/memories/{id}
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "memory id is required")
		return
	}

	if err := h.memory.Delete(r.Context(), id); err != nil {
		h.logger.Error("delete failed", "id", id, "error", err)
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    map[string]string{"deleted": id},
	})
}

// Health handles GET /health
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]string{
			"status": "healthy",
		},
	})
}

// writeJSON writes a JSON response
func (h *Handler) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// writeError writes an error response
func (h *Handler) writeError(w http.ResponseWriter, status int, message string) {
	h.writeJSON(w, status, Response{
		Success: false,
		Error:   message,
	})
}
