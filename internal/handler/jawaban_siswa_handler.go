package handler

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"momo-be/internal/service"
	"momo-be/internal/sse"
	appErrors "momo-be/internal/errors"
)

type JawabanSiswaHandler struct {
	service *service.JawabanSiswaService
	hub     *sse.Hub
}

func NewJawabanSiswaHandler(service *service.JawabanSiswaService, hub *sse.Hub) *JawabanSiswaHandler {
	return &JawabanSiswaHandler{service: service, hub: hub}
}

type submitJawabanRequest struct {
	SoalID        uint   `json:"soal_id" binding:"required"`
	JawabanMentah string `json:"jawaban_mentah" binding:"required"`
}

func (h *JawabanSiswaHandler) SubmitJawaban(c *gin.Context) {
	siswaID, ok := getUintFromContext(c, "siswa_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, appErrors.NewClientError(
			appErrors.CodeUnauthorized, "Token siswa tidak valid"))
		return
	}

	var req submitJawabanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, appErrors.NewClientError(
			appErrors.CodeMissingField, "soal_id dan jawaban_mentah wajib diisi"))
		return
	}

	jawaban, err := h.service.SubmitJawaban(siswaID, req.SoalID, req.JawabanMentah)
	if err != nil {
		log.Printf("[jawaban] gagal submit: %v", err)
		
		errMsg := err.Error()
		var resp appErrors.ErrorResponse
		
		switch {
		// AI errors
		case strings.Contains(errMsg, "gagal menghubungi AI Service") || strings.Contains(errMsg, "timeout"):
			resp = appErrors.NewAIError(appErrors.CodeAIUnavailable,
				"Gagal menghubungi AI Service untuk mengevaluasi jawaban. Silakan coba lagi.")
		case strings.Contains(errMsg, "AI Service"):
			resp = appErrors.NewAIError(appErrors.CodeAIProcessingFailed,
				"AI gagal mengevaluasi jawaban. Silakan coba lagi.")
		
		// Client errors
		case strings.Contains(errMsg, "sudah pernah dijawab"):
			resp = appErrors.NewClientError(appErrors.CodeAlreadyAnswered,
				"Soal UTS/UAS hanya boleh dijawab sekali")
		case strings.Contains(errMsg, "tidak ditugaskan"):
			resp = appErrors.NewClientError(appErrors.CodeForbidden,
				"Soal ini tidak ditugaskan untuk kelas Anda")
		case strings.Contains(errMsg, "tidak ditemukan"):
			resp = appErrors.NewClientError(appErrors.CodeNotFound,
				errMsg)
		
		// Server errors
		default:
			resp = appErrors.NewServerError(appErrors.CodeInternalError,
				"Terjadi kesalahan sistem saat menyimpan jawaban")
		}
		
		c.JSON(appErrors.GetHTTPStatus(resp.Code), resp)
		return
	}

	if h.hub != nil {
		h.hub.BroadcastToGuru("jawaban-submitted", map[string]interface{}{
			"siswa_id": siswaID,
			"soal_id":  req.SoalID,
			"benar":    jawaban.Benar,
		})
	}

	c.JSON(http.StatusCreated, jawaban)
}