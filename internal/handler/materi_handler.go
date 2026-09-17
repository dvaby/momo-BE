package handler

import (
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"momo-be/internal/service"
	"momo-be/internal/sse"
)

type MateriHandler struct {
	service *service.MateriService
	hub     *sse.Hub
}

func NewMateriHandler(service *service.MateriService, hub *sse.Hub) *MateriHandler {
	return &MateriHandler{service: service, hub: hub}
}

func (h *MateriHandler) ListMateri(c *gin.Context) {
	guruID, ok := getUintFromContext(c, "guru_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	modulIDParam := c.Param("id")
	modulID, err := strconv.ParseUint(modulIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID modul tidak valid"})
		return
	}

	materiList, err := h.service.GetByModulID(uint(modulID), guruID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"jumlah": len(materiList),
		"data":   materiList,
	})
}

func (h *MateriHandler) CreateMateriManual(c *gin.Context) {
	guruID, ok := getUintFromContext(c, "guru_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	modulIDParam := c.Param("id")
	modulID, err := strconv.ParseUint(modulIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID modul tidak valid"})
		return
	}

	var req struct {
		Judul  string `json:"judul" binding:"required"`
		Konten string `json:"konten" binding:"required"`
		Urutan int    `json:"urutan"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Judul dan konten materi wajib diisi"})
		return
	}

	materi, err := h.service.CreateManual(uint(modulID), guruID, req.Urutan, req.Judul, req.Konten)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Materi berhasil ditambahkan",
		"data":    materi,
	})
}

func (h *MateriHandler) UpdateMateri(c *gin.Context) {
	guruID, ok := getUintFromContext(c, "guru_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	materiIDParam := c.Param("id")
	materiID, err := strconv.ParseUint(materiIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID materi tidak valid"})
		return
	}

	var req struct {
		Judul  string `json:"judul" binding:"required"`
		Konten string `json:"konten" binding:"required"`
		Urutan int    `json:"urutan"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Judul dan konten materi wajib diisi"})
		return
	}

	materi, err := h.service.Update(uint(materiID), guruID, req.Judul, req.Konten, req.Urutan)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Materi berhasil diperbarui",
		"data":    materi,
	})
}

func (h *MateriHandler) DeleteMateri(c *gin.Context) {
	guruID, ok := getUintFromContext(c, "guru_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	materiIDParam := c.Param("id")
	materiID, err := strconv.ParseUint(materiIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID materi tidak valid"})
		return
	}

	if err := h.service.Delete(uint(materiID), guruID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Materi berhasil dihapus"})
}

func (h *MateriHandler) UploadMateri(c *gin.Context) {
	guruID, ok := getUintFromContext(c, "guru_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	modulIDParam := c.Param("id")
	modulID, err := strconv.ParseUint(modulIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID modul tidak valid"})
		return
	}

	// Validasi kepemilikan modul SEBELUM proses file
	if err := h.service.ValidateModulOwnership(uint(modulID), guruID); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File PDF wajib dilampirkan dengan field 'file'"})
		return
	}

	// Validasi ekstensi
	if !strings.HasSuffix(strings.ToLower(file.Filename), ".pdf") {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Hanya file PDF yang diperbolehkan",
			"code":  "INVALID_FILE_TYPE",
		})
		return
	}

	// Validasi ukuran
	const maxSize = 25 * 1024 * 1024 // 25MB
	if file.Size > maxSize {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Ukuran file terlalu besar. Maksimal 25MB.",
			"code":  "FILE_TOO_LARGE",
		})
		return
	}

	// Simpan file ke temp (karena service butuh path string)
	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuka file"})
		return
	}
	defer src.Close()

	tempFile, err := os.CreateTemp("", "materi-*.pdf")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat file sementara"})
		return
	}

	_, err = io.Copy(tempFile, src)
	tempFile.Close()
	if err != nil {
		os.Remove(tempFile.Name())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan file sementara"})
		return
	}

	tempFilePath := tempFile.Name()
	defer os.Remove(tempFilePath)

	// Proses materi (signature asli: modulID, pdfFilePath)
	materiList, err := h.service.ProcessAndSaveMateri(uint(modulID), tempFilePath)
	if err != nil {
		log.Printf("[materi] gagal memproses materi untuk modul %d: %v", modulID, err)
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error": err.Error(),
			"code":  "MATERI_PROCESSING_FAILED",
		})
		return
	}

	log.Printf("[materi] berhasil memproses %d materi untuk modul %d", len(materiList), modulID)

	// Emit event ke unified hub (bonus, tidak blocking)
	if h.hub != nil {
		h.hub.BroadcastToGuru("materi-ready", map[string]interface{}{
			"modul_id": modulID,
			"jumlah":   len(materiList),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Materi berhasil diproses",
		"jumlah":  len(materiList),
		"data":    materiList,
	})
}
