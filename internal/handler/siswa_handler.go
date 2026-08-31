package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"momo-be/internal/service"
)

type SiswaHandler struct {
	service *service.SiswaService
}

func NewSiswaHandler(service *service.SiswaService) *SiswaHandler {
	return &SiswaHandler{service: service}
}

type daftarSiswaRequest struct {
	Nama string `json:"nama" binding:"required"`
}

func (h *SiswaHandler) DaftarkanSiswa(c *gin.Context) {
	kelasIDParam := c.Param("id")
	kelasID, err := strconv.ParseUint(kelasIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID kelas tidak valid"})
		return
	}

	var req daftarSiswaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	siswa, err := h.service.DaftarkanSiswa(uint(kelasID), req.Nama)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, siswa)
}

// joinSiswaRequest adalah body yang dikirim saat siswa "join" lewat suara:
// sebut kode kelas, lalu sebut nama.
type joinSiswaRequest struct {
	KodeKelas string `json:"kode_kelas" binding:"required"`
	Nama      string `json:"nama" binding:"required"`
}

// joinSiswaResponse adalah bentuk balasan ke FE: data siswa yang berhasil
// join, plus token JWT yang harus disimpan FE dan dikirim ulang di
// request-request berikutnya (ambil soal, submit jawaban, dst).
type joinSiswaResponse struct {
	SiswaID uint   `json:"siswa_id"`
	KelasID uint   `json:"kelas_id"`
	Nama    string `json:"nama"`
	Token   string `json:"token"`
}

func (h *SiswaHandler) JoinSiswa(c *gin.Context) {
	var req joinSiswaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	siswa, token, err := h.service.JoinSiswa(req.KodeKelas, req.Nama)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, joinSiswaResponse{
		SiswaID: siswa.ID,
		KelasID: siswa.KelasID,
		Nama:    siswa.Nama,
		Token:   token,
	})
}