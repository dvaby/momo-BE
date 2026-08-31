package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"momo-be/internal/model"
	"momo-be/internal/service"
)

type GuruHandler struct {
	guruService service.GuruService
}

func NewGuruHandler(guruService service.GuruService) *GuruHandler {
	return &GuruHandler{guruService: guruService}
}

func (h *GuruHandler) Register(c *gin.Context) {
	var req model.RegisterGuruRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	guru, err := h.guruService.Register(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "pendaftaran guru berhasil",
		"data":    guru,
	})
}

func (h *GuruHandler) Login(c *gin.Context) {
	var req model.LoginGuruRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.guruService.Login(&req)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "login berhasil",
		"data":    resp,
	})
}