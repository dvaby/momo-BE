package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	appErrors "momo-be/internal/errors"
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
}

// SubmitTutor — POST /api/v1/tutor (KHUSUS SISWA)
// SYNCHRONOUS sejak 19 Sept: response berisi LANGSUNG teks balasan tutor.
// (Event stream tutor-reply tetap dipancarkan untuk demo/listener publik.)
func (h *TutorHandler) SubmitTutor(c *gin.Context) {
	siswaID, ok := getUintFromContext(c, "siswa_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, appErrors.NewClientError(appErrors.CodeUnauthorized, "Token siswa tidak valid"))
		return
	}

	var req tutorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, appErrors.NewClientError(appErrors.CodeMissingField, "Field 'pesan' wajib diisi"))
		return
	}

	kelasNama := req.KelasNama
	if kelasNama == "" {
		kelasNama = "Siswa"
	}

	balasan, jobID, err := h.service.ProcessTutor(siswaID, kelasNama, req.Pesan)
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
		"message": "Balasan tutor diterima",
		"job_id":  jobID,
		"balasan": balasan,
	})
}