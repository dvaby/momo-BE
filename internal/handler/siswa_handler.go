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