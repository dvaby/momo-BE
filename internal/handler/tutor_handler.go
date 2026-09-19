package handler

import (
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
	SessionID string `json:"session_id"`
	// KodeKelas tidak perlu dikirim FE lagi, backend otomatis extract dari teks pesan
}

// SubmitTutor — POST /api/v1/chat
func (h *TutorHandler) SubmitTutor(c *gin.Context) {
	var req tutorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, appErrors.NewClientError(appErrors.CodeMissingField, "Field 'pesan' wajib diisi"))
		return
	}

	// Pegangan obrolan: pakai yang dikirim FE, atau buat baru untuk pengunjung pertama
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = job.GenerateID("anon")
	}

	kelasNama := req.KelasNama
	if kelasNama == "" {
		kelasNama = "Siswa"
	}

	// Panggil service dengan 3 argument (sessionID, kelasNama, pesan)
	// Return value cuma 2 (res, err) karena job_id sudah di-handle internal
	res, err := h.service.ProcessTutor(sessionID, kelasNama, req.Pesan)
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

	// Response bersih ke FE: tanpa job_id
	c.JSON(http.StatusOK, gin.H{
		"message":    "Balasan tutor diterima",
		"session_id": sessionID,
		"balasan":    res.Balasan,
		"fase":       res.Fase,
		"extract": gin.H{
			"nama":       res.ExtractNama,
			"kode_kelas": res.ExtractKode,
		},
		"join":       res.Join,      // berisi token HANYA jika kode kelas valid
		"join_error": res.JoinError, // terisi jika kode tidak valid
	})
}