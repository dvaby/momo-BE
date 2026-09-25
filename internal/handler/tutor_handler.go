package handler

import (
	"net/http"

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
}

// SubmitTutor — POST /api/v1/chat
func (h *TutorHandler) SubmitTutor(c *gin.Context) {
	var req tutorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, appErrors.NewClientError(appErrors.CodeMissingField, "Field 'pesan' wajib diisi"))
		return
	}

	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = job.GenerateID("anon")
	}

	kelasNama := req.KelasNama
	if kelasNama == "" {
		kelasNama = "Siswa"
	}

	res, err := h.service.ProcessTutor(sessionID, kelasNama, req.Pesan)
	if err != nil {
		// FIX UX: untuk aplikasi suara, lebih baik FE menerima 200 dengan balasan
		// ramah yang bisa langsung di-TTS, daripada 503 yang bikin FE stuck/error toast.
		c.JSON(http.StatusOK, gin.H{
			"message":    "Balasan tutor diterima (fallback)",
			"job_id":     job.GenerateID("fallback"),
			"session_id": sessionID,
			"balasan":    "Maaf, aku sedang agak sibuk sebentar. Coba ucapkan lagi ya, aku pasti dengarkan.",
			"fase":       "belajar",
			"is_fallback": true,
			"extract": gin.H{
				"nama":       "",
				"kode_kelas": "",
			},
			"join":       nil,
			"join_error": "",
		})
		return
	}

	// Response ke FE: job_id dan session_id tetap ada
	c.JSON(http.StatusOK, gin.H{
		"message":    "Balasan tutor diterima",
		"job_id":     res.JobID,
		"session_id": sessionID,
		"balasan":    res.Balasan,
		"fase":       res.Fase,
		"extract": gin.H{
			"nama":       res.ExtractNama,
			"kode_kelas": res.ExtractKode,
		},
		"join":       res.Join,
		"join_error": res.JoinError,
	})
}