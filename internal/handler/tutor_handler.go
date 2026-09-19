package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	appErrors "momo-be/internal/errors"
	"momo-be/internal/job"
	"momo-be/internal/service"
)

type TutorHandler struct {
	service *service.TutorService
}

func NewTutorHandler(service *service.TutorService) *TutorHandler {
	return &TutorHandler{service: service}
}

type tutorRequest struct {
	Pesan     string `json:"pesan" binding:"required"`
	KelasNama string `json:"kelas_nama"`
	SessionID string `json:"session_id"` // opsional: kunci memory percakapan
}

// SubmitTutor — POST /api/v1/chat (AUTH OPSIONAL sejak 19 Sept)
// - Pakai token siswa  → session otomatis "siswa-<id>" (memory nyambung ke akun)
// - Tanpa token        → session dari body session_id (atau dibuat baru)
// Response SYNCHRONOUS berisi teks balasan siap TTS.
func (h *TutorHandler) SubmitTutor(c *gin.Context) {
	var req tutorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, appErrors.NewClientError(appErrors.CodeMissingField, "Field 'pesan' wajib diisi"))
		return
	}

	// Identitas opsional: ada token siswa → pakai session siswa; tidak ada → anonim
	sessionID := req.SessionID
	if siswaID, ok := getUintFromContext(c, "siswa_id"); ok {
		sessionID = fmt.Sprintf("siswa-%d", siswaID)
	} else if sessionID == "" {
		sessionID = job.GenerateID("anon")
	}

	kelasNama := req.KelasNama
	if kelasNama == "" {
		kelasNama = "Siswa"
	}

	balasan, jobID, err := h.service.ProcessTutor(sessionID, kelasNama, req.Pesan)
	if err != nil {
		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, "timeout"):
			c.JSON(http.StatusServiceUnavailable, appErrors.NewAIError(appErrors.CodeAITimeout, "Momo terlalu lama merespons. Silakan coba lagi."))
		case strings.Contains(errMsg, "AI Service"), strings.Contains(errMsg, "AI tutor"):
			c.JSON(http.StatusServiceUnavailable, appErrors.NewAIError(appErrors.CodeAIUnavailable, "AI tutor sedang tidak tersedia. Silakan coba lagi dalam beberapa saat."))
		default:
			c.JSON(http.StatusInternalServerError, appErrors.NewServerError(appErrors.CodeInternalError, errMsg))
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Balasan tutor diterima",
		"job_id":     jobID,
		"session_id": sessionID, // FE WAJIB simpan & kirim balik tiap giliran
		"balasan":    balasan,
	})
}