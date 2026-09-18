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
// Fire-and-forget: return ACK <1 detik, balasan tutor dikirim via stream event 'tutor-reply'.
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

	jobID, err := h.service.SubmitTutorRequest(siswaID, kelasNama, req.Pesan)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "AI Service") {
			c.JSON(http.StatusServiceUnavailable, appErrors.NewAIError(
				appErrors.CodeAIUnavailable,
				"AI tutor sedang tidak tersedia. Silakan coba lagi dalam beberapa saat."))
			return
		}
		c.JSON(http.StatusInternalServerError, appErrors.NewServerError(
			appErrors.CodeInternalError, errMsg))
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message": "Permintaan tutor diterima, balasan akan dikirim via stream",
		"job_id":  jobID,
	})
}