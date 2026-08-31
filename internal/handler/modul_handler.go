package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"momo-be/internal/service"
)

type ModulHandler struct {
	modulService *service.ModulService
}

func NewModulHandler(modulService *service.ModulService) *ModulHandler {
	return &ModulHandler{modulService: modulService}
}

type createModulRequest struct {
	Nama      string `json:"nama" binding:"required"`
	Deskripsi string `json:"deskripsi"`
}

func (h *ModulHandler) CreateModul(c *gin.Context) {
	guruIDVal, exists := c.Get("guru_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Akses khusus guru"})
		return
	}
	guruID := guruIDVal.(uint)

	var req createModulRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	modul, err := h.modulService.Create(guruID, req.Nama, req.Deskripsi)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, modul)
}

func (h *ModulHandler) GetAllModuls(c *gin.Context) {
	guruIDVal, exists := c.Get("guru_id")
	if exists {
		guruID := guruIDVal.(uint)
		moduls, err := h.modulService.GetByGuruID(guruID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, moduls)
		return
	}

	moduls, err := h.modulService.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, moduls)
}

func (h *ModulHandler) GetModulByID(c *gin.Context) {
	guruIDVal, exists := c.Get("guru_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Akses khusus guru"})
		return
	}
	guruID := guruIDVal.(uint)

	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID modul tidak valid"})
		return
	}

	modul, err := h.modulService.GetByIDAndGuruID(uint(id), guruID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Modul tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, ToModulDetailResponse(*modul))
}
