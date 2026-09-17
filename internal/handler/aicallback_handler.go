package handler

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"momo-be/internal/job"
)

type AICallbackHandler struct {
	registry    *job.Registry
	internalKey string
}

func NewAICallbackHandler(registry *job.Registry, internalKey string) *AICallbackHandler {
	return &AICallbackHandler{
		registry:    registry,
		internalKey: internalKey,
	}
}

// Handle menerima callback dari AI Service setelah job selesai.
// Autentikasi via query param `token` yang disisipkan di callback_url oleh backend.
// AI Service cukup POST ke URL yang kami kirim — tidak perlu ubah code mereka.
func (h *AICallbackHandler) Handle(c *gin.Context) {
	// Validasi token dari query param (bukan header)
	token := c.Query("token")
	if token != h.internalKey || h.internalKey == "" {
		log.Printf("[ai-callback] REJECTED: token tidak valid (dapat '%s')", token)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token tidak valid"})
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "gagal baca body"})
		return
	}

	var req job.CallbackRequest
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body bukan JSON valid"})
		return
	}

	if req.JobID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "job_id wajib diisi"})
		return
	}

	log.Printf("[ai-callback] ACCEPTED: job=%s tipe=%s status=%s", req.JobID, req.Tipe, req.Status)

	switch req.Status {
	case "success":
		if err := h.registry.Complete(req.JobID, req.Hasil); err != nil {
			log.Printf("[ai-callback] warning: %v", err)
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
	case "failed":
		if err := h.registry.Fail(req.JobID, req.ErrorMessage); err != nil {
			log.Printf("[ai-callback] warning: %v", err)
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "status wajib 'success' atau 'failed'"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "job_id": req.JobID})
}
