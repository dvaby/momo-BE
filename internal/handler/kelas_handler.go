package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"momo-be/internal/service"
)

type KelasHandler struct {
	service *service.KelasService
}

func NewKelasHandler(service *service.KelasService) *KelasHandler {
	return &KelasHandler{service: service}
}

type createKelasRequest struct {
	NamaKelas string `json:"nama_kelas" binding:"required"`
}

func (h *KelasHandler) CreateKelas(c *gin.Context) {
	var req createKelasRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	kelas, err := h.service.CreateKelas(req.NamaKelas)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, kelas)
}

func (h *KelasHandler) GetKelasByID(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID kelas tidak valid"})
		return
	}

	kelas, err := h.service.GetKelasByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Kelas tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, kelas)
}