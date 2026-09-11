package handler

import (
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"

	"momo-be/internal/model"
	"momo-be/internal/service"
)

type GuruHandler struct {
	guruService service.GuruService
	feVerifyURL string
}

func NewGuruHandler(guruService service.GuruService, feVerifyURL string) *GuruHandler {
	return &GuruHandler{guruService: guruService, feVerifyURL: feVerifyURL}
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
		"message": "pendaftaran berhasil, silakan cek email kamu untuk verifikasi akun",
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

func (h *GuruHandler) VerifyEmail(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.Redirect(http.StatusFound, h.feVerifyURL+"?status=error&message=Token+tidak+disertakan")
		return
	}

	if err := h.guruService.VerifyEmail(token); err != nil {
		c.Redirect(http.StatusFound, h.feVerifyURL+"?status=error&message="+url.QueryEscape(err.Error()))
		return
	}

	c.Redirect(http.StatusFound, h.feVerifyURL+"?status=success")
}
