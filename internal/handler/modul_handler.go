package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"momo-be/internal/service"
)

type ModulHandler struct {
	service *service.ModulService
}

func NewModulHandler(service *service.ModulService) *ModulHandler {
	return &ModulHandler{service: service}
}

type createModulRequest struct {
	Judul     string `json:"judul" binding:"required"`
	Deskripsi string `json:"deskripsi"`
}

func (h *ModulHandler) CreateModul(c *gin.Context) {
	var req createModulRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	modul, err := h.service.CreateModul(req.Judul, req.Deskripsi)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat modul"})
		return
	}

	c.JSON(http.StatusCreated, modul)
}

func (h *ModulHandler) GetAllModul(c *gin.Context) {
	modulList, err := h.service.GetAllModul()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data modul"})
		return
	}
	c.JSON(http.StatusOK, modulList)
}

func (h *ModulHandler) GetModulByID(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID modul tidak valid"})
		return
	}

	modul, err := h.service.GetModulByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Modul tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, modul)
}