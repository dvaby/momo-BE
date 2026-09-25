package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"momo-be/internal/service"
)

type ProgressHandler struct {
	service *service.ProgressService
}

func NewProgressHandler(service *service.ProgressService) *ProgressHandler {
	return &ProgressHandler{service: service}
}

// GetProgressKelas handler untuk GET /api/v1/kelas/:id/progress
// 🔥 FIX: pakai "id" BUKAN "kelas_id" biar gak konflik sama route lain
func (h *ProgressHandler) GetProgressKelas(c *gin.Context) {
	// ✅ Ambil parameter "id" dari URL
	kelasIDStr := c.Param("id")
	kelasID, err := strconv.ParseUint(kelasIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "INVALID_KELAS_ID",
			"message": "ID kelas tidak valid",
		})
		return
	}

	// Verifikasi guru yang login adalah pemilik kelas
	guruIDInterface, exists := c.Get("guru_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    "UNAUTHORIZED",
			"message": "Guru tidak terautentikasi",
		})
		return
	}
	guruID, ok := guruIDInterface.(uint)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "INTERNAL_ERROR",
			"message": "Invalid guru ID type",
		})
		return
	}

	// Get progress data
	ringkasan, siswa := h.service.GetProgressKelas(uint(kelasID))

	// Verifikasi ownership (guru harus pemilik kelas)
	// ... (tambah logic verifikasi kalau perlu)

	c.JSON(http.StatusOK, gin.H{
		"ringkasan": ringkasan,
		"siswa":     siswa,
		"guru_id":   guruID,
		"kelas_id":  kelasID,
	})
}