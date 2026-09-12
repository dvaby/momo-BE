package handler

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"momo-be/internal/service"
)

type MateriHandler struct {
	service *service.MateriService
}

func NewMateriHandler(service *service.MateriService) *MateriHandler {
	return &MateriHandler{service: service}
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

	if err := h.service.ValidateModulOwnership(uint(modulID), guruID); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File PDF wajib dilampirkan dengan field 'file'"})
		return
	}

	// --- Validasi file: maks 25MB (PDF materi bisa besar) & wajib .pdf ---
	const maxSize = 25 * 1024 * 1024 // 25MB
	if file.Size > maxSize {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Ukuran file terlalu besar. Maksimal 25MB.",
			"code":  "FILE_TOO_LARGE",
		})
		return
	}
	if ext := strings.ToLower(filepath.Ext(file.Filename)); ext != ".pdf" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Hanya file PDF yang diperbolehkan",
			"code":  "INVALID_FILE_TYPE",
		})
		return
	}
	// --------------------------------------------------------------------

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
	defer os.Remove(tempFilePath) // aman di-defer karena sekarang synchronous

	// ============================================================
	// SYNCHRONOUS: request DITAHAN sampai AI selesai merangkum.
	// FE tidak akan pernah menerima pesan "sedang diproses" lagi.
	// ============================================================
	materiList, err := h.service.ProcessAndSaveMateri(uint(modulID), tempFilePath)
	if err != nil {
		log.Printf("[materi] gagal memproses materi untuk modul %d: %v", modulID, err)

		errMsg := "Gagal memproses materi dari PDF. "
		switch {
		case strings.Contains(err.Error(), "ekstrak PDF"):
			errMsg += "File PDF tidak bisa dibaca. Pastikan PDF berisi teks, bukan hasil scan gambar."
		case strings.Contains(err.Error(), "AI Service"), strings.Contains(err.Error(), "timeout"):
			errMsg += "Layanan AI terlalu lama memproses atau tidak tersedia. Coba lagi atau gunakan PDF yang lebih kecil."
		case strings.Contains(err.Error(), "tidak menemukan konten"):
			errMsg += "AI tidak menemukan konten materi yang valid. Pastikan PDF berisi materi pembelajaran, bukan soal."
		default:
			errMsg += err.Error()
		}

		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error": errMsg,
			"code":  "MATERI_PROCESSING_FAILED",
		})
		return
	}

	log.Printf("[materi] berhasil memproses %d materi untuk modul %d", len(materiList), modulID)

	// Hasil rangkuman ASLI langsung dikirim ke FE
	c.JSON(http.StatusOK, gin.H{
		"message": "Materi berhasil diproses",
		"jumlah":  len(materiList),
		"data":    materiList,
	})
}
