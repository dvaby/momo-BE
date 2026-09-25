package handler

import (
	"net/http"
	"net/url"
	"strings"

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
		errMsg := "Mohon lengkapi semua field dengan benar"
		errStr := err.Error()
		if strings.Contains(errStr, "Nama") || strings.Contains(errStr, "nama") {
			errMsg = "Nama wajib diisi (minimal 2 karakter)"
		} else if strings.Contains(errStr, "Email") || strings.Contains(errStr, "email") {
			errMsg = "Format email tidak valid"
		} else if strings.Contains(errStr, "Password") || strings.Contains(errStr, "password") {
			errMsg = "Password minimal 6 karakter"
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
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
		errMsg := "Mohon lengkapi semua field dengan benar"
		errStr := err.Error()
		if strings.Contains(errStr, "Email") || strings.Contains(errStr, "email") {
			errMsg = "Format email tidak valid"
		} else if strings.Contains(errStr, "Password") || strings.Contains(errStr, "password") {
			errMsg = "Password wajib diisi"
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
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

// GetProfile - GET /api/v1/guru/profile
func (h *GuruHandler) GetProfile(c *gin.Context) {
	guruIDUint, exists := c.Get("guru_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    "UNAUTHORIZED",
			"message": "Token tidak valid atau tidak ditemukan",
			"source":  "client",
		})
		return
	}
	guruID := guruIDUint.(uint)

	guru, err := h.guruService.GetProfile(guruID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    "NOT_FOUND",
			"message": err.Error(),
			"source":  "client",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Profil berhasil diambil",
		"data":    guru,
	})
}

// UpdateProfile - PUT /api/v1/guru/profile
func (h *GuruHandler) UpdateProfile(c *gin.Context) {
	guruIDUint, exists := c.Get("guru_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    "UNAUTHORIZED",
			"message": "Token tidak valid atau tidak ditemukan",
			"source":  "client",
		})
		return
	}
	guruID := guruIDUint.(uint)

	var req model.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := "Mohon lengkapi field dengan benar"
		errStr := err.Error()
		if strings.Contains(errStr, "Nama") || strings.Contains(errStr, "nama") {
			errMsg = "Nama minimal 2 karakter"
		} else if strings.Contains(errStr, "Password") || strings.Contains(errStr, "password") {
			errMsg = "Password minimal 6 karakter"
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "INVALID_REQUEST",
			"message": errMsg,
			"source":  "client",
		})
		return
	}

	guru, err := h.guruService.UpdateProfile(guruID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "INVALID_REQUEST",
			"message": err.Error(),
			"source":  "client",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Profil berhasil diperbarui",
		"data":    guru,
	})
}