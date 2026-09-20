package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"momo-be/internal/service"
)

// ToolsHandler melayani eksekusi tool yang diminta AI Service (internal only)
type ToolsHandler struct {
	tutor       *service.TutorService
	internalKey string
}

func NewToolsHandler(tutor *service.TutorService, internalKey string) *ToolsHandler {
	return &ToolsHandler{tutor: tutor, internalKey: internalKey}
}

type toolsExecRequest struct {
	SessionID string             `json:"session_id" binding:"required"`
	ToolCalls []service.ToolCall `json:"tool_calls" binding:"required"`
}

func (h *ToolsHandler) Execute(c *gin.Context) {
	if h.internalKey == "" || c.Query("token") != h.internalKey {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token tidak valid"})
		return
	}

	var req toolsExecRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	results := h.tutor.ExecuteToolCalls(req.SessionID, req.ToolCalls)
	c.JSON(http.StatusOK, gin.H{"results": results})
}