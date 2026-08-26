package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"momo-be/internal/service"
)

type JawabanSiswaHandler struct {
	service *service.JawabanSiswaService
}

func NewJawabanSiswaHandler(service *service.JawabanSiswaService) *JawabanSiswaHandler {
	return &JawabanSiswaHandler{service: service}
}

// submitJawabanRequest adalah body yang dikirim saat siswa submit
// jawaban untuk 1 soal. soal_id datang dari body (bukan URL param),
// karena siswa sedang mengerjakan 1 soal spesifik pada satu waktu.
type submitJawabanRequest struct {
	SoalID        uint   `json:"soal_id" binding:"required"`
	JawabanMentah string `json:"jawaban_mentah" binding:"required"`
}

func (h *JawabanSiswaHandler) SubmitJawaban(c *gin.Context) {
	// siswa_id TIDAK diambil dari body/param request, tapi dari context
	// yang sudah diisi AuthMiddleware setelah verifikasi token. Ini
	// mencegah siswa "menjawab atas nama" siswa lain dengan cara
	// mengubah siswa_id di body secara manual.
	siswaIDRaw, exists := c.Get("siswa_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "identitas siswa tidak ditemukan"})
		return
	}
	siswaID, ok := siswaIDRaw.(uint)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca identitas siswa"})
		return
	}

	var req submitJawabanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	jawaban, err := h.service.SubmitJawaban(siswaID, req.SoalID, req.JawabanMentah)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, jawaban)
}