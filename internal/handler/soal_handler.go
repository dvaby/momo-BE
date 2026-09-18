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
	appErrors "momo-be/internal/errors"
)

type SoalHandler struct {
	service *service.SoalService
	hub     *sse.Hub
}

func NewSoalHandler(service *service.SoalService, hub *sse.Hub) *SoalHandler {
	return &SoalHandler{service: service, hub: hub}
}

func (h *SoalHandler) UploadSoal(c *gin.Context) {
	guruID, ok := getUintFromContext(c, "guru_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, appErrors.NewClientError(
			appErrors.CodeUnauthorized, "Token guru tidak valid"))
		return
	}

	modulIDParam := c.Param("id")
	modulID, err := strconv.ParseUint(modulIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, appErrors.NewClientError(
			appErrors.CodeInvalidFormat, "ID modul tidak valid"))
		return
	}

	jenisParam := c.Query("jenis")
	jenis := model.JenisSoal(jenisParam)
	if jenis != model.JenisSoalHarian && jenis != model.JenisSoalUTS && jenis != model.JenisSoalUAS {
		c.JSON(http.StatusBadRequest, appErrors.NewClientError(
			appErrors.CodeMissingField, "Query param 'jenis' wajib: harian, uts, atau uas"))
		return
	}

	if err := h.service.ValidateModulOwnership(uint(modulID), guruID); err != nil {
		c.JSON(http.StatusForbidden, appErrors.NewClientError(
			appErrors.CodeForbidden, err.Error()))
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, appErrors.NewClientError(
			appErrors.CodeMissingField, "File PDF wajib dilampirkan dengan field 'file'"))
		return
	}

	const maxSize = 5 * 1024 * 1024
	if file.Size > maxSize {
		c.JSON(http.StatusBadRequest, appErrors.NewClientError(
			appErrors.CodeInvalidFile, "Ukuran file terlalu besar. Maksimal 5MB"))
		return
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".pdf" {
		c.JSON(http.StatusBadRequest, appErrors.NewClientError(
			appErrors.CodeInvalidFile, "Hanya file PDF yang diperbolehkan"))
		return
	}

	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, appErrors.NewServerError(
			appErrors.CodeInternalError, "Gagal membuka file"))
		return
	}
	defer src.Close()

	tempFile, err := os.CreateTemp("", "soal-*.pdf")
	if err != nil {
		c.JSON(http.StatusInternalServerError, appErrors.NewServerError(
			appErrors.CodeInternalError, "Gagal membuat file sementara"))
		return
	}

	_, err = io.Copy(tempFile, src)
	tempFile.Close()
	if err != nil {
		os.Remove(tempFile.Name())
		c.JSON(http.StatusInternalServerError, appErrors.NewServerError(
			appErrors.CodeInternalError, "Gagal menyimpan file sementara"))
		return
	}

	tempFilePath := tempFile.Name()
	defer os.Remove(tempFilePath)

	soalList, err := h.service.ProcessAndSaveSoal(uint(modulID), jenis, tempFilePath, guruID)
	if err != nil {
		log.Printf("[soal] gagal memproses soal untuk modul %d: %v", modulID, err)
		
		errMsg := err.Error()
		var resp appErrors.ErrorResponse
		
		switch {
		// AI Service errors
		case strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "menunggu callback"):
			resp = appErrors.NewAIError(appErrors.CodeAITimeout,
				"AI Service terlalu lama memproses. Silakan coba lagi.")
		case strings.Contains(errMsg, "gagal menghubungi AI Service") || strings.Contains(errMsg, "tidak tersedia"):
			resp = appErrors.NewAIError(appErrors.CodeAIUnavailable,
				"AI Service sedang tidak tersedia. Silakan coba beberapa saat lagi.")
		case strings.Contains(errMsg, "gagal parse hasil") || strings.Contains(errMsg, "tidak dikenali"):
			resp = appErrors.NewAIError(appErrors.CodeAIInvalidResponse,
				"AI Service mengembalikan response tidak valid.")
		case strings.Contains(errMsg, "AI Service") || strings.Contains(errMsg, "tidak menemukan"):
			resp = appErrors.NewAIError(appErrors.CodeAIProcessingFailed,
				"AI gagal mengekstrak soal dari PDF. Pastikan PDF berisi soal pilihan ganda.")
		
		// Client errors
		case strings.Contains(errMsg, "ekstrak PDF"):
			resp = appErrors.NewClientError(appErrors.CodeInvalidFile,
				"File PDF tidak bisa dibaca. Pastikan PDF berisi teks, bukan hasil scan gambar.")
		
		// Server errors
		case strings.Contains(errMsg, "database"):
			resp = appErrors.NewServerError(appErrors.CodeDatabaseError,
				"Gagal menyimpan soal ke database.")
		default:
			resp = appErrors.NewServerError(appErrors.CodeInternalError,
				"Terjadi kesalahan sistem saat memproses soal.")
		}
		
		c.JSON(appErrors.GetHTTPStatus(resp.Code), resp)
		return
	}

	responseData := make([]SoalResponse, 0, len(soalList))
	for _, soal := range soalList {
		responseData = append(responseData, ToSoalResponse(soal))
	}

	log.Printf("[soal] berhasil memproses %d soal untuk modul %d", len(responseData), modulID)

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

// Method-method lain (GetSoalByModul, GetSoalByModulForGuru, CreateSoalManual, UpdateSoal, DeleteSoal)
// TIDAK BERUBAH - biarkan seperti sebelumnya
func (h *SoalHandler) GetSoalByModul(c *gin.Context) {
	kelasID, ok := getUintFromContext(c, "kelas_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, appErrors.NewClientError(
			appErrors.CodeUnauthorized, "Akses khusus siswa"))
		return
	}

	modulIDParam := c.Param("id")
	modulID, err := strconv.ParseUint(modulIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, appErrors.NewClientError(
			appErrors.CodeInvalidFormat, "ID modul tidak valid"))
		return
	}

	jenisParam := c.Query("jenis")
	jenis := model.JenisSoal(jenisParam)
	if jenis != model.JenisSoalHarian && jenis != model.JenisSoalUTS && jenis != model.JenisSoalUAS {
		c.JSON(http.StatusBadRequest, appErrors.NewClientError(
			appErrors.CodeMissingField, "Query param 'jenis' wajib: harian, uts, atau uas"))
		return
	}

	soalList, err := h.service.GetSoalByModulAndJenisForSiswa(uint(modulID), kelasID, jenis)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "tidak ditugaskan") {
			c.JSON(http.StatusForbidden, appErrors.NewClientError(
				appErrors.CodeForbidden, errMsg))
		} else {
			c.JSON(http.StatusInternalServerError, appErrors.NewServerError(
				appErrors.CodeDatabaseError, errMsg))
		}
		return
	}

	if len(soalList) == 0 {
		c.JSON(http.StatusNotFound, appErrors.NewClientError(
			appErrors.CodeNotFound, "Belum ada soal untuk modul dan jenis ini"))
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

func (h *SoalHandler) GetSoalByModulForGuru(c *gin.Context) {
	guruID, ok := getUintFromContext(c, "guru_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, appErrors.NewClientError(
			appErrors.CodeUnauthorized, "Token guru tidak valid"))
		return
	}

	modulID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, appErrors.NewClientError(
			appErrors.CodeInvalidFormat, "ID modul tidak valid"))
		return
	}

	jenisParam := c.Query("jenis")
	jenis := model.JenisSoal(jenisParam)
	if jenisParam != "" && jenis != model.JenisSoalHarian && jenis != model.JenisSoalUTS && jenis != model.JenisSoalUAS {
		c.JSON(http.StatusBadRequest, appErrors.NewClientError(
			appErrors.CodeInvalidFormat, "Query param 'jenis' wajib: harian, uts, atau uas"))
		return
	}

	soalList, err := h.service.GetByModulIDAndJenis(uint(modulID), guruID, jenis)
	if err != nil {
		c.JSON(http.StatusForbidden, appErrors.NewClientError(
			appErrors.CodeForbidden, err.Error()))
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

func (h *SoalHandler) CreateSoalManual(c *gin.Context) {
	guruID, ok := getUintFromContext(c, "guru_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, appErrors.NewClientError(
			appErrors.CodeUnauthorized, "Token guru tidak valid"))
		return
	}

	modulID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, appErrors.NewClientError(
			appErrors.CodeInvalidFormat, "ID modul tidak valid"))
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
		c.JSON(http.StatusBadRequest, appErrors.NewClientError(
			appErrors.CodeMissingField, "Semua field (jenis, pertanyaan, pilihan A-D, kunci jawaban) wajib diisi"))
		return
	}

	soal, err := h.service.CreateManual(
		uint(modulID), guruID,
		model.JenisSoal(req.Jenis),
		req.Pertanyaan, req.PilihanA, req.PilihanB, req.PilihanC, req.PilihanD,
		req.KunciJawaban,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, appErrors.NewClientError(
			appErrors.CodeInvalidRequest, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Soal berhasil ditambahkan",
		"data":    ToSoalResponse(*soal),
	})
}

func (h *SoalHandler) UpdateSoal(c *gin.Context) {
	guruID, ok := getUintFromContext(c, "guru_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, appErrors.NewClientError(
			appErrors.CodeUnauthorized, "Token guru tidak valid"))
		return
	}

	soalID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, appErrors.NewClientError(
			appErrors.CodeInvalidFormat, "ID soal tidak valid"))
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
		c.JSON(http.StatusBadRequest, appErrors.NewClientError(
			appErrors.CodeMissingField, "Semua field (pertanyaan, pilihan A-D, kunci jawaban) wajib diisi"))
		return
	}

	soal, err := h.service.Update(
		uint(soalID), guruID,
		req.Pertanyaan, req.PilihanA, req.PilihanB, req.PilihanC, req.PilihanD,
		req.KunciJawaban, model.JenisSoal(req.Jenis),
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, appErrors.NewClientError(
			appErrors.CodeInvalidRequest, err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Soal berhasil diperbarui",
		"data":    ToSoalResponse(*soal),
	})
}

func (h *SoalHandler) DeleteSoal(c *gin.Context) {
	guruID, ok := getUintFromContext(c, "guru_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, appErrors.NewClientError(
			appErrors.CodeUnauthorized, "Token guru tidak valid"))
		return
	}

	soalID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, appErrors.NewClientError(
			appErrors.CodeInvalidFormat, "ID soal tidak valid"))
		return
	}

	if err := h.service.Delete(uint(soalID), guruID); err != nil {
		c.JSON(http.StatusBadRequest, appErrors.NewClientError(
			appErrors.CodeInvalidRequest, err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Soal berhasil dihapus"})
}