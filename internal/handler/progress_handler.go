package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"momo-be/internal/service"
)

type ProgressHandler struct{ svc *service.ProgressService }

func NewProgressHandler(svc *service.ProgressService) *ProgressHandler {
	return &ProgressHandler{svc: svc}
}

// GetProgressKelas — GET /api/v1/kelas/:kelas_id/progress
func (h *ProgressHandler) GetProgressKelas(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("kelas_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kelas_id tidak valid"})
		return
	}
	ring, siswa := h.svc.GetProgressKelas(uint(id))
	c.JSON(http.StatusOK, gin.H{"ringkasan": ring, "siswa": siswa})
}