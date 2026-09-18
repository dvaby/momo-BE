package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	appErrors "momo-be/internal/errors"
	"momo-be/internal/service"
)

type SiswaHandler struct {
	service *service.SiswaService
}

func NewSiswaHandler(service *service.SiswaService) *SiswaHandler {
	return &SiswaHandler{service: service}
}

type daftarSiswaRequest struct {
	Nama string `json:"nama" binding:"required"`
}

type joinSiswaRequest struct {
	KodeKelas string `json:"kode_kelas" binding:"required"`
	Nama      string `json:"nama" binding:"required"`
}

type joinSiswaResponse struct {
	SiswaID uint   `json:"siswa_id"`
	KelasID uint   `json:"kelas_id"`
	Nama    string `json:"nama"`
	Token   string `json:"token"`
}

// DaftarkanSiswa — POST /api/v1/kelas/:id/siswa (KHUSUS GURU)
func (h *SiswaHandler) DaftarkanSiswa(c *gin.Context) {
	guruID, ok := getUintFromContext(c, "guru_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, appErrors.NewClientError(appErrors.CodeUnauthorized, "Akses khusus guru"))
		return
	}

	kelasID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, appErrors.NewClientError(appErrors.CodeInvalidFormat, "ID kelas tidak valid"))
		return
	}

	var req daftarSiswaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, appErrors.NewClientError(appErrors.CodeMissingField, "Field 'nama' wajib diisi"))
		return
	}

	siswa, err := h.service.DaftarkanSiswa(uint(kelasID), guruID, req.Nama)
	if err != nil {
		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, "sudah terdaftar"):
			c.JSON(http.StatusConflict, appErrors.NewClientError(appErrors.CodeDuplicate, errMsg))
		case strings.Contains(errMsg, "tidak ditemukan"), strings.Contains(errMsg, "akses"):
			c.JSON(http.StatusForbidden, appErrors.NewClientError(appErrors.CodeForbidden, errMsg))
		default:
			c.JSON(http.StatusInternalServerError, appErrors.NewServerError(appErrors.CodeDatabaseError, errMsg))
		}
		return
	}

	c.JSON(http.StatusCreated, siswa)
}

// JoinSiswa — POST /api/v1/join (PUBLIC)
func (h *SiswaHandler) JoinSiswa(c *gin.Context) {
	var req joinSiswaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, appErrors.NewClientError(appErrors.CodeMissingField, "kode_kelas dan nama wajib diisi"))
		return
	}

	siswa, token, err := h.service.JoinSiswa(req.KodeKelas, req.Nama)
	if err != nil {
		c.JSON(http.StatusNotFound, appErrors.NewClientError(appErrors.CodeNotFound, err.Error()))
		return
	}

	c.JSON(http.StatusOK, joinSiswaResponse{
		SiswaID: siswa.ID,
		KelasID: siswa.KelasID,
		Nama:    siswa.Nama,
		Token:   token,
	})
}

// GetKelasSaya — GET /api/v1/siswa/kelas-saya (KHUSUS SISWA)
func (h *SiswaHandler) GetKelasSaya(c *gin.Context) {
	siswaID, ok := getUintFromContext(c, "siswa_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, appErrors.NewClientError(appErrors.CodeUnauthorized, "Token siswa tidak valid"))
		return
	}
	kelasID, ok := getUintFromContext(c, "kelas_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, appErrors.NewClientError(appErrors.CodeUnauthorized, "Token siswa tidak valid"))
		return
	}

	result, err := h.service.GetKelasSaya(siswaID, kelasID)
	if err != nil {
		c.JSON(http.StatusForbidden, appErrors.NewClientError(appErrors.CodeForbidden, err.Error()))
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetModulDetailForSiswa — GET /api/v1/siswa/modul/:id (KHUSUS SISWA, BARU)
func (h *SiswaHandler) GetModulDetailForSiswa(c *gin.Context) {
	siswaID, ok := getUintFromContext(c, "siswa_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, appErrors.NewClientError(appErrors.CodeUnauthorized, "Token siswa tidak valid"))
		return
	}
	kelasID, ok := getUintFromContext(c, "kelas_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, appErrors.NewClientError(appErrors.CodeUnauthorized, "Token siswa tidak valid"))
		return
	}

	modulID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, appErrors.NewClientError(appErrors.CodeInvalidFormat, "ID modul tidak valid"))
		return
	}

	result, err := h.service.GetModulDetailForSiswa(siswaID, kelasID, uint(modulID))
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "tidak ditugaskan") || strings.Contains(errMsg, "akses ditolak") {
			c.JSON(http.StatusForbidden, appErrors.NewClientError(appErrors.CodeForbidden, errMsg))
			return
		}
		c.JSON(http.StatusInternalServerError, appErrors.NewServerError(appErrors.CodeDatabaseError, errMsg))
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetMateriForSiswa — GET /api/v1/siswa/modul/:id/materi (KHUSUS SISWA)
func (h *SiswaHandler) GetMateriForSiswa(c *gin.Context) {
	siswaID, ok := getUintFromContext(c, "siswa_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, appErrors.NewClientError(appErrors.CodeUnauthorized, "Token siswa tidak valid"))
		return
	}
	kelasID, ok := getUintFromContext(c, "kelas_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, appErrors.NewClientError(appErrors.CodeUnauthorized, "Token siswa tidak valid"))
		return
	}

	modulID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, appErrors.NewClientError(appErrors.CodeInvalidFormat, "ID modul tidak valid"))
		return
	}

	materiList, err := h.service.GetMateriBelajar(siswaID, kelasID, uint(modulID))
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "tidak ditugaskan") || strings.Contains(errMsg, "akses ditolak") {
			c.JSON(http.StatusForbidden, appErrors.NewClientError(appErrors.CodeForbidden, errMsg))
			return
		}
		c.JSON(http.StatusInternalServerError, appErrors.NewServerError(appErrors.CodeDatabaseError, errMsg))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"jumlah": len(materiList),
		"data":   materiList,
	})
}