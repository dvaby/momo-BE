package handler

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"momo-be/internal/service"
	"momo-be/internal/sse"
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
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req submitJawabanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "soal_id dan jawaban_mentah wajib diisi"})
		return
	}

	// Method di service adalah SubmitJawaban (bukan SubmitDanEvaluasiJawaban)
	jawaban, err := h.service.SubmitJawaban(siswaID, req.SoalID, req.JawabanMentah)
	if err != nil {
		log.Printf("[jawaban] gagal submit: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Emit event ke unified hub (opsional, untuk analytics real-time)
	if h.hub != nil {
		h.hub.BroadcastToGuru("jawaban-submitted", map[string]interface{}{
			"siswa_id": siswaID,
			"soal_id":  req.SoalID,
			"benar":    jawaban.Benar,
		})
	}

	c.JSON(http.StatusCreated, jawaban)
}
