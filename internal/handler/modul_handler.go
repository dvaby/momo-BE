package handler

import (
	"io"
	"net/http"
	"strconv"
	"time"

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

// StreamStatusModul adalah endpoint SSE untuk real-time update status modul
func (h *ModulHandler) StreamStatusModul(c *gin.Context) {
	modulIDParam := c.Param("id")
	modulID, err := strconv.ParseUint(modulIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID modul tidak valid"})
		return
	}

	// Set header untuk SSE (Server-Sent Events)
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	// Streaming loop
	c.Stream(func(w io.Writer) bool {
		// PERBAIKAN: Gunakan h.modulService dan method GetByID
		modul, err := h.modulService.GetByID(uint(modulID))
		if err != nil {
			c.SSEvent("error", "Modul tidak ditemukan")
			return false // Stop stream
		}

		// Jika soal sudah ada (AI selesai), kirim event 'done'
		if modul.Soal != nil && len(modul.Soal) > 0 {
			c.SSEvent("done", modul)
			return false // Stop stream karena selesai
		}

		// Jika belum selesai, kirim event 'processing'
		c.SSEvent("processing", "AI sedang membaca soal...")

		// Tunggu 3 detik sebelum cek lagi agar tidak membebani database
		time.Sleep(3 * time.Second)
		return true // Lanjutkan stream
	})
}

// --- DTO untuk Update ---

type updateModulRequest struct {
	Nama      string `json:"nama"`
	Deskripsi string `json:"deskripsi"`
}

// UpdateModul — PUT /api/v1/modul/:id
func (h *ModulHandler) UpdateModul(c *gin.Context) {
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

	var req updateModulRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validasi: minimal salah satu field harus diisi
	if req.Nama == "" && req.Deskripsi == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Minimal salah satu field (nama atau deskripsi) harus diisi"})
		return
	}

	modul, err := h.modulService.Update(uint(id), guruID, req.Nama, req.Deskripsi)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Modul berhasil diperbarui",
		"data":    modul,
	})
}

// DeleteModul — DELETE /api/v1/modul/:id
func (h *ModulHandler) DeleteModul(c *gin.Context) {
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

	err = h.modulService.Delete(uint(id), guruID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Modul berhasil dihapus beserta semua materi dan soal di dalamnya",
	})
}
