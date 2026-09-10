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

// Request DTOs
type createKelasRequest struct {
	Nama          string `json:"nama" binding:"required"`
	MataPelajaran string `json:"mata_pelajaran" binding:"required"`
}

type updateKelasRequest struct {
	Nama          string `json:"nama"`
	MataPelajaran string `json:"mata_pelajaran"`
}

type assignModulRequest struct {
	ModulID uint `json:"modul_id" binding:"required"`
}

// CREATE - Buat kelas baru
func (h *KelasHandler) CreateKelas(c *gin.Context) {
	guruIDVal, exists := c.Get("guru_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Akses khusus guru"})
		return
	}
	guruID := guruIDVal.(uint)

	var req createKelasRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	kelas, err := h.service.CreateKelas(guruID, req.Nama, req.MataPelajaran)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, kelas)
}

// READ - Dapatkan semua kelas milik guru
func (h *KelasHandler) GetKelasGuru(c *gin.Context) {
	guruIDVal, exists := c.Get("guru_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Akses khusus guru"})
		return
	}
	guruID := guruIDVal.(uint)

	kelass, err := h.service.GetKelasByGuruID(guruID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, kelass)
}

// READ - Dapatkan kelas by ID
func (h *KelasHandler) GetKelasByID(c *gin.Context) {
	guruIDVal, exists := c.Get("guru_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Akses khusus guru"})
		return
	}
	guruID := guruIDVal.(uint)

	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID kelas tidak valid"})
		return
	}

	kelas, err := h.service.GetKelasByID(uint(id), guruID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, kelas)
}

// UPDATE - Update kelas (METHOD BARU - ini yang hilang!)
func (h *KelasHandler) UpdateKelas(c *gin.Context) {
	guruIDVal, exists := c.Get("guru_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Akses khusus guru"})
		return
	}
	guruID := guruIDVal.(uint)

	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID kelas tidak valid"})
		return
	}

	var req updateKelasRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	kelas, err := h.service.UpdateKelas(uint(id), guruID, req.Nama, req.MataPelajaran)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Kelas berhasil diperbarui",
		"data":    kelas,
	})
}

// DELETE - Hapus kelas (METHOD BARU - ini yang hilang!)
func (h *KelasHandler) DeleteKelas(c *gin.Context) {
	guruIDVal, exists := c.Get("guru_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Akses khusus guru"})
		return
	}
	guruID := guruIDVal.(uint)

	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID kelas tidak valid"})
		return
	}

	err = h.service.DeleteKelas(uint(id), guruID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Kelas berhasil dihapus"})
}

// Assign modul ke kelas
func (h *KelasHandler) AssignModul(c *gin.Context) {
	guruIDVal, exists := c.Get("guru_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Akses khusus guru"})
		return
	}
	guruID := guruIDVal.(uint)

	kelasIDParam := c.Param("id")
	kelasID, err := strconv.ParseUint(kelasIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID kelas tidak valid"})
		return
	}

	var req assignModulRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = h.service.AssignModul(uint(kelasID), req.ModulID, guruID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Modul berhasil ditautkan ke kelas"})
}

// Remove modul dari kelas (METHOD BARU - ini yang hilang!)
func (h *KelasHandler) RemoveModul(c *gin.Context) {
	guruIDVal, exists := c.Get("guru_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Akses khusus guru"})
		return
	}
	guruID := guruIDVal.(uint)

	kelasIDParam := c.Param("id")
	kelasID, err := strconv.ParseUint(kelasIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID kelas tidak valid"})
		return
	}

	modulIDParam := c.Param("modul_id")
	modulID, err := strconv.ParseUint(modulIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID modul tidak valid"})
		return
	}

	err = h.service.RemoveModul(uint(kelasID), uint(modulID), guruID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Modul berhasil dihapus dari kelas"})
}