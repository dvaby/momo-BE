package handler

import (
	"net/http"
	"strconv"

	"momo-be/internal/service"

	"github.com/gin-gonic/gin"
)

type ModulHandler struct {
	modulService service.ModulService
}

func NewModulHandler(modulService service.ModulService) *ModulHandler {
	return &ModulHandler{modulService}
}

func (h *ModulHandler) CreateModul(c *gin.Context) {
	var req struct {
		Judul     string `json:"judul" binding:"required"`
		Deskripsi string `json:"deskripsi"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Ambil guru_id dari context (diset oleh GuruAuthMiddleware)
	guruID, exists := c.Get("guru_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Akses ditolak, token tidak valid"})
		return
	}

	modul, err := h.modulService.CreateModul(req.Judul, req.Deskripsi, guruID.(uint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, modul)
}

func (h *ModulHandler) GetAllModuls(c *gin.Context) {
	guruID, exists := c.Get("guru_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Akses ditolak, token tidak valid"})
		return
	}

	moduls, err := h.modulService.GetAllModuls(guruID.(uint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, moduls)
}

func (h *ModulHandler) GetModulByID(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID modul tidak valid"})
		return
	}

	guruID, exists := c.Get("guru_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Akses ditolak, token tidak valid"})
		return
	}

	modul, err := h.modulService.GetModulByID(uint(id), guruID.(uint))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// MEMPERTAHANKAN FIX BUG #1: Menggunakan DTO agar kunci jawaban aman
	c.JSON(http.StatusOK, ToModulDetailResponse(*modul))
}