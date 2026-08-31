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
	// FIX: Ambil guru_id untuk otorisasi akses kelas
	guruID, ok := getUintFromContext(c, "guru_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	kelasIDParam := c.Param("id")
	kelasID, err := strconv.ParseUint(kelasIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID kelas tidak valid"})
		return
	}

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

	jenis := c.Query("jenis")

	// FIX: Pass guruID ke layer service untuk memvalidasi apakah kelasID ini milik guruID terkait
	rekap, err := h.service.GetRekapNilai(uint(kelasID), uint(modulID), jenis, guruID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, rekap)
}