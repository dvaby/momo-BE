package handler

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"momo-be/internal/job"
	"momo-be/internal/sse"
)

type AICallbackHandler struct {
	registry    *job.Registry
	internalKey string
	hub         *sse.Hub
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
	// 1. Validasi token
	token := c.Query("token")
	if token != h.internalKey || h.internalKey == "" {
		log.Printf("[ai-callback] REJECTED: token tidak valid (dapat '%s')", token)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token tidak valid"})
		return
	}

	// 2. Baca body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "gagal baca body"})
		return
	}

	// 3. Parse body
	var req job.CallbackRequest
	if err := json.Unmarshal(body, &req); err != nil {
		log.Printf("[ai-callback] RAW BODY (unparseable): %s", string(body))
		c.JSON(http.StatusBadRequest, gin.H{"error": "body bukan JSON valid"})
		return
	}

	if req.JobID == "" {
		log.Printf("[ai-callback] RAW BODY (tanpa job_id): %s", string(body))
		c.JSON(http.StatusBadRequest, gin.H{"error": "job_id wajib diisi"})
		return
	}

	// OBSERVABILITY (permintaan tim AI Service):
	// - Job failed: log RAW BODY LENGKAP apa adanya + error_message eksplisit,
	//   biar bisa cross-check dengan log Vercel mereka.
	// - Job success: log ringkas seperti sebelumnya (hasil dipotong 300 karakter).
	if req.Status == "failed" {
		log.Printf("[ai-callback] RAW BODY (failed): %s", string(body))
		log.Printf("[ai-callback] FAILED job=%s error_message=%q", req.JobID, req.ErrorMessage)
	} else {
		rawHasil := string(req.Hasil)
		if len(rawHasil) > 300 {
			rawHasil = rawHasil[:300] + "..."
		}
		log.Printf("[ai-callback] ACCEPTED: job=%s tipe=%s status=%s, hasil=%s", req.JobID, req.Tipe, req.Status, rawHasil)
	}

	// 4. Switch berdasarkan status
	switch req.Status {
	case "success":
		if err := h.registry.Complete(req.JobID, req.Hasil); err != nil {
			log.Printf("[ai-callback] warning: %v", err)
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		// Khusus tutor: emit event tutor-reply ke siswa spesifik + fallback public listener
		if req.Tipe == string(job.JobTypeTutor) && h.hub != nil {
			h.handleTutorSuccess(req)
		}

	case "failed":
		// Jangan biarkan error_message kosong mengalir ke user/log
		errMsg := req.ErrorMessage
		if errMsg == "" {
			errMsg = "AI gagal tanpa keterangan (error_message kosong dari AI Service)"
		}
		if err := h.registry.Fail(req.JobID, errMsg); err != nil {
			log.Printf("[ai-callback] warning: %v", err)
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		// Kalau tutor gagal, tetap emit event dengan balasan error ramah
		if req.Tipe == string(job.JobTypeTutor) && h.hub != nil {
			h.handleTutorFailed(req)
		}

	default:
		log.Printf("[ai-callback] RAW BODY (status invalid): %s", string(body))
		c.JSON(http.StatusBadRequest, gin.H{"error": "status wajib 'success' atau 'failed'"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "job_id": req.JobID})
}

// handleTutorSuccess emit event tutor-reply setelah callback sukses.
// Route ke siswa spesifik jika ada siswa_id, fallback ke listener public untuk demo.
func (h *AICallbackHandler) handleTutorSuccess(req job.CallbackRequest) {
	jawaban, parseErr := job.ParseTutorResult(req.Hasil)
	if parseErr != nil {
		log.Printf("[ai-callback] tutor parse error: %v", parseErr)
		jawaban = "Maaf, aku belum bisa memahami pesanmu. Coba lagi ya."
	}

	siswaID := h.extractSiswaID(req.JobID)

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
		// Fallback untuk session anonim (demo/listener public)
		h.hub.BroadcastToSiswa("tutor-reply", map[string]interface{}{
			"job_id":  req.JobID,
			"balasan": jawaban,
		})
		shortJawaban := jawaban
		if len(shortJawaban) > 50 {
			shortJawaban = shortJawaban[:50] + "..."
		}
		log.Printf("[ai-callback] tutor-reply emitted ke public listener: %s", shortJawaban)
	}
}

// handleTutorFailed emit event tutor-reply denCallbackRequestgan balasan error ramah
// supaya FE tetap bisa speak pesan yang sopan alih-alih stuck.
func (h *AICallbackHandler) handleTutorFailed(req job.CallbackRequest) {
	siswaID := h.extractSiswaID(req.JobID)
	payload := map[string]interface{}{
		"job_id":  req.JobID,
		"balasan": "Maaf, aku sedang mengalami kesulitan. Silakan coba lagi.",
		"error":   true,
	}

	if siswaID > 0 {
		h.hub.BroadcastToSiswaByID("tutor-reply", payload, siswaID)
		log.Printf("[ai-callback] tutor-reply (failed) emitted ke siswa %d", siswaID)
	} else {
		h.hub.BroadcastToSiswa("tutor-reply", payload)
		log.Printf("[ai-callback] tutor-reply (failed) emitted ke public listener")
	}
}

// extractSiswaID mengambil siswa_id dari SessionID di registry.
// Session anonim (prefix "anon_") return 0 tanpa warning.
// Session non-anonim yang gagal di-parse → log warning.
func (h *AICallbackHandler) extractSiswaID(jobID string) uint {
	jobInfo := h.registry.Get(jobID)
	if jobInfo == nil || jobInfo.SessionID == "" {
		return 0
	}

	// Session anonim: tidak ada siswa_id, tidak perlu warning
	if strings.HasPrefix(jobInfo.SessionID, "anon_") {
		return 0
	}

	id, err := strconv.ParseUint(jobInfo.SessionID, 10, 64)
	if err != nil {
		log.Printf("[ai-callback] WARNING: siswa_id tidak ditemukan di registry untuk job %s (session_id=%s)", jobID, jobInfo.SessionID)
		return 0
	}
	return uint(id)
}