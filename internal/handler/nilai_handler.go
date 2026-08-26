package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"momo-be/internal/service"
)

type NilaiHandler struct {
	service *service.NilaiService
}

func NewNilaiHandler(service *service.NilaiService) *NilaiHandler {
	return &NilaiHandler{service: service}
}

func (h *NilaiHandler) GetRekapNilai(c *gin.Context) {
	// 1. Ambil ID Kelas dari path parameter /kelas/:id/nilai
	kelasIDParam := c.Param("id")
	kelasID, err := strconv.ParseUint(kelasIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID kelas tidak valid"})
		return
	}

	// 2. Ambil query parameter modul_id (?modul_id=X)
	modulIDParam := c.Query("modul_id")
	if modulIDParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query parameter 'modul_id' wajib diisi"})
		return
	}

	modulID, err := strconv.ParseUint(modulIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query parameter 'modul_id' harus berupa angka"})
		return
	}

	// 3. Ambil query parameter jenis (?jenis=Y), bersifat opsional
	jenis := c.Query("jenis")

	// 4. Panggil Service layer
	rekap, err := h.service.GetRekapNilai(uint(kelasID), uint(modulID), jenis)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 5. Kembalikan response HTTP 200 OK dengan slice of NilaiSiswa
	c.JSON(http.StatusOK, rekap)
}