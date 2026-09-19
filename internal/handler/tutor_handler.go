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
	SessionID string `json:"session_id"`
	KodeKelas string `json:"kode_kelas"` // hint: kode 6 digit terakhir yang disebut siswa (direkam FE)
}

// SubmitTutor — POST /api/v1/chat (AUTH OPSIONAL)
func (h *TutorHandler) SubmitTutor(c *gin.Context) {
	var req tutorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, appErrors.NewClientError(appErrors.CodeMissingField, "Field 'pesan' wajib diisi"))
		return
	}

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

	res, jobID, err := h.service.ProcessTutor(sessionID, kelasNama, req.Pesan, req.KodeKelas)
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