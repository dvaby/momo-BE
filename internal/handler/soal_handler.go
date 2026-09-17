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

	"momo-be/internal/model"
	"momo-be/internal/service"
	"momo-be/internal/sse"
)

type SoalHandler struct {
	service *service.SoalService
	hub     *sse.Hub
}

func NewSoalHandler(service *service.SoalService, hub *sse.Hub) *SoalHandler {
	return &SoalHandler{service: service, hub: hub}
}

// SoalResponse dan ToSoalResponse sudah ada di dto.go, tidak perlu dideklarasikan ulang

func (h *SoalHandler) UploadSoal(c *gin.Context) {
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

	jenisParam := c.Query("jenis")
	jenis := model.JenisSoal(jenisParam)
	if jenis != model.JenisSoalHarian && jenis != model.JenisSoalUTS && jenis != model.JenisSoalUAS {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query param 'jenis' wajib salah satu dari: harian, uts, uas"})
		return
	}

	// Validasi kepemilikan Modul dilakukan SEKARANG (synchronous),
	// supaya guru langsung tahu kalau ditolak, tanpa nunggu proses PDF/AI.
	if err := h.service.ValidateModulOwnership(uint(modulID), guruID); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File PDF wajib dilampirkan dengan field 'file'"})
		return
	}

	// --- Validasi Ukuran & Tipe File ---
	const maxSize = 5 * 1024 * 1024 // 5MB
	if file.Size > maxSize {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Ukuran file terlalu besar. Maksimal 5MB.",
			"code":  "FILE_TOO_LARGE",
		})
		return
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".pdf" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Hanya file PDF yang diperbolehkan",
			"code":  "INVALID_FILE_TYPE",
		})
		return
	}
	// -----------------------------------

	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuka file"})
		return
	}
	defer src.Close()

	tempFile, err := os.CreateTemp("", "soal-*.pdf")
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

	soalList, err := h.service.ProcessAndSaveSoal(uint(modulID), jenis, tempFilePath, guruID)
	if err != nil {
		log.Printf("[soal] gagal memproses soal untuk modul %d: %v", modulID, err)

		errMsg := "Gagal memproses soal dari PDF. "
		switch {
		case strings.Contains(err.Error(), "ekstrak PDF"):
			errMsg += "File PDF tidak bisa dibaca. Pastikan PDF berisi teks, bukan hasil scan gambar."
		case strings.Contains(err.Error(), "AI Service"), strings.Contains(err.Error(), "timeout"):
			errMsg += "Layanan AI terlalu lama memproses atau tidak tersedia. Coba lagi atau gunakan PDF yang lebih kecil."
		case strings.Contains(err.Error(), "tidak menemukan"):
			errMsg += "AI tidak menemukan soal pilihan ganda yang valid di PDF ini."
		default:
			errMsg += err.Error()
		}

		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error": errMsg,
			"code":  "SOAL_PROCESSING_FAILED",
		})
		return
	}

	// Konversi ke SoalResponse agar kunci_jawaban TIDAK bocor ke FE
	responseData := make([]SoalResponse, 0, len(soalList))
	for _, soal := range soalList {
		responseData = append(responseData, ToSoalResponse(soal))
	}

	log.Printf("[soal] berhasil memproses %d soal untuk modul %d", len(responseData), modulID)

	// Emit event ke unified hub (bonus, tidak blocking)
	if h.hub != nil {
		h.hub.BroadcastToGuru("soal-ready", map[string]interface{}{
			"modul_id": modulID,
			"jenis":    jenis,
			"jumlah":   len(responseData),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Soal berhasil diproses",
		"jenis":   jenis,
		"jumlah":  len(responseData),
		"data":    responseData,
	})
}

func (h *SoalHandler) GetSoalByModul(c *gin.Context) {
	kelasID, ok := getUintFromContext(c, "kelas_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Akses khusus siswa"})
		return
	}

	modulIDParam := c.Param("id")
	modulID, err := strconv.ParseUint(modulIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID modul tidak valid"})
		return
	}

	jenisParam := c.Query("jenis")
	jenis := model.JenisSoal(jenisParam)
	if jenis != model.JenisSoalHarian && jenis != model.JenisSoalUTS && jenis != model.JenisSoalUAS {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query param 'jenis' wajib salah satu dari: harian, uts, uas"})
		return
	}

	soalList, err := h.service.GetSoalByModulAndJenisForSiswa(uint(modulID), kelasID, jenis)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	if len(soalList) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Belum ada soal untuk modul dan jenis ini"})
		return
	}

	responseData := make([]SoalResponse, 0, len(soalList))
	for _, soal := range soalList {
		responseData = append(responseData, ToSoalResponse(soal))
	}

	c.JSON(http.StatusOK, gin.H{
		"jenis":  jenis,
		"jumlah": len(responseData),
		"data":   responseData,
	})
}

// GetSoalByModulForGuru — GET /api/v1/modul/:id/soal/list
func (h *SoalHandler) GetSoalByModulForGuru(c *gin.Context) {
	guruID, ok := getUintFromContext(c, "guru_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	modulID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID modul tidak valid"})
		return
	}

	jenisParam := c.Query("jenis")
	jenis := model.JenisSoal(jenisParam)
	if jenisParam != "" && jenis != model.JenisSoalHarian && jenis != model.JenisSoalUTS && jenis != model.JenisSoalUAS {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query param 'jenis' wajib salah satu dari: harian, uts, uas"})
		return
	}

	soalList, err := h.service.GetByModulIDAndJenis(uint(modulID), guruID, jenis)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	responseData := make([]SoalResponse, 0, len(soalList))
	for _, soal := range soalList {
		responseData = append(responseData, ToSoalResponse(soal))
	}

	c.JSON(http.StatusOK, gin.H{
		"jenis":  jenisParam,
		"jumlah": len(responseData),
		"data":   responseData,
	})
}

// CreateSoalManual — POST /api/v1/modul/:id/soal/manual
func (h *SoalHandler) CreateSoalManual(c *gin.Context) {
	guruID, ok := getUintFromContext(c, "guru_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	modulID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID modul tidak valid"})
		return
	}

	var req struct {
		Jenis        string `json:"jenis" binding:"required"`
		Pertanyaan   string `json:"pertanyaan" binding:"required"`
		PilihanA     string `json:"pilihan_a" binding:"required"`
		PilihanB     string `json:"pilihan_b" binding:"required"`
		PilihanC     string `json:"pilihan_c" binding:"required"`
		PilihanD     string `json:"pilihan_d" binding:"required"`
		KunciJawaban string `json:"kunci_jawaban" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Semua field (jenis, pertanyaan, pilihan A-D, kunci jawaban) wajib diisi"})
		return
	}

	soal, err := h.service.CreateManual(
		uint(modulID), guruID,
		model.JenisSoal(req.Jenis),
		req.Pertanyaan, req.PilihanA, req.PilihanB, req.PilihanC, req.PilihanD,
		req.KunciJawaban,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Soal berhasil ditambahkan",
		"data":    ToSoalResponse(*soal),
	})
}

// UpdateSoal — PUT /api/v1/soal/:id
func (h *SoalHandler) UpdateSoal(c *gin.Context) {
	guruID, ok := getUintFromContext(c, "guru_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	soalID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID soal tidak valid"})
		return
	}

	var req struct {
		Jenis        string `json:"jenis"`
		Pertanyaan   string `json:"pertanyaan" binding:"required"`
		PilihanA     string `json:"pilihan_a" binding:"required"`
		PilihanB     string `json:"pilihan_b" binding:"required"`
		PilihanC     string `json:"pilihan_c" binding:"required"`
		PilihanD     string `json:"pilihan_d" binding:"required"`
		KunciJawaban string `json:"kunci_jawaban" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Semua field (pertanyaan, pilihan A-D, kunci jawaban) wajib diisi"})
		return
	}

	soal, err := h.service.Update(
		uint(soalID), guruID,
		req.Pertanyaan, req.PilihanA, req.PilihanB, req.PilihanC, req.PilihanD,
		req.KunciJawaban, model.JenisSoal(req.Jenis),
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Soal berhasil diperbarui",
		"data":    ToSoalResponse(*soal),
	})
}

// DeleteSoal — DELETE /api/v1/soal/:id
func (h *SoalHandler) DeleteSoal(c *gin.Context) {
	guruID, ok := getUintFromContext(c, "guru_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	soalID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID soal tidak valid"})
		return
	}

	if err := h.service.Delete(uint(soalID), guruID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Soal berhasil dihapus"})
}
