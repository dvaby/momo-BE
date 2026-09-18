package handler

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"momo-be/internal/job"
	"momo-be/internal/sse"
)

type AICallbackHandler struct {
	registry    *job.Registry
	internalKey string
	hub         *sse.Hub // BARU: untuk emit tutor-reply
}

func NewAICallbackHandler(registry *job.Registry, internalKey string, hub *sse.Hub) *AICallbackHandler {
	return &AICallbackHandler{
		registry:    registry,
		internalKey: internalKey,
		hub:         hub,
	}
}

// Handle menerima callback dari AI Service setelah job selesai.
// Autentikasi via query param `token` yang disisipkan di callback_url oleh backend.
func (h *AICallbackHandler) Handle(c *gin.Context) {
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

	rawHasil := string(req.Hasil)
	if len(rawHasil) > 300 {
		rawHasil = rawHasil[:300] + "..."
	}
	log.Printf("[ai-callback] ACCEPTED: job=%s tipe=%s status=%s, hasil=%s", req.JobID, req.Tipe, req.Status, rawHasil)

	switch req.Status {
	case "success":
		if err := h.registry.Complete(req.JobID, req.Hasil); err != nil {
			log.Printf("[ai-callback] warning: %v", err)
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		// BARU: kalau job tutor, emit event tutor-reply ke siswa spesifik
		if req.Tipe == string(job.JobTypeTutor) && h.hub != nil {
			jawaban, parseErr := job.ParseTutorResult(req.Hasil)
			if parseErr != nil {
				log.Printf("[ai-callback] tutor parse error: %v", parseErr)
				jawaban = "Maaf, aku belum bisa memahami pesanmu. Coba lagi ya."
			}

			// Ambil siswa_id dari registry (disimpan di SessionID)
			var siswaID uint
			if jobInfo := h.registry.Get(req.JobID); jobInfo != nil && jobInfo.SessionID != "" {
				if id, err := strconv.ParseUint(jobInfo.SessionID, 10, 64); err == nil {
					siswaID = uint(id)
				}
			}

			if siswaID > 0 {
				h.hub.BroadcastToSiswaByID("tutor-reply", map[string]interface{}{
					"job_id":  req.JobID,
					"balasan": jawaban,
				}, siswaID)
				shortJawaban := jawaban
				if len(shortJawaban) > 50 {
					shortJawaban = shortJawaban[:50] + "..."
				}
				log.Printf("[ai-callback] tutor-reply emitted ke siswa %d: %s", siswaID, shortJawaban)
			} else {
				log.Printf("[ai-callback] WARNING: siswa_id tidak ditemukan di registry untuk job %s", req.JobID)
			}
		}

	case "failed":
		if err := h.registry.Fail(req.JobID, req.ErrorMessage); err != nil {
			log.Printf("[ai-callback] warning: %v", err)
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		// Kalau tutor gagal, tetap emit event dengan pesan error
		if req.Tipe == string(job.JobTypeTutor) && h.hub != nil {
			if jobInfo := h.registry.Get(req.JobID); jobInfo != nil && jobInfo.SessionID != "" {
				if id, err := strconv.ParseUint(jobInfo.SessionID, 10, 64); err == nil {
					h.hub.BroadcastToSiswaByID("tutor-reply", map[string]interface{}{
						"job_id": req.JobID,
						"balasan": "Maaf, aku sedang mengalami kesulitan. Silakan coba lagi.",
						"error":  true,
					}, uint(id))
				}
			}
		}

	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "status wajib 'success' atau 'failed'"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "job_id": req.JobID})
}