package handler

import (
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"momo-be/internal/model"
	"momo-be/internal/service"
)

type SoalHandler struct {
	service *service.SoalService
}

func NewSoalHandler(service *service.SoalService) *SoalHandler {
	return &SoalHandler{service: service}
}

func (h *SoalHandler) UploadSoal(c *gin.Context) {
	// FIX: Ambil guru_id dari context
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

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File PDF wajib dilampirkan dengan field 'file'"})
		return
	}

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
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	_, err = io.Copy(tempFile, src)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan file sementara"})
		return
	}

	// FIX: Pass guruID ke service untuk memvalidasi kepemilikan modul_id
	soalList, err := h.service.ProcessAndSaveSoal(uint(modulID), jenis, tempFile.Name(), guruID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// FIX: Mapping ke DTO untuk mencegah kebocoran KunciJawaban pada raw DB Model
	responseData := make([]SoalResponse, 0, len(soalList))
	for _, soal := range soalList {
		responseData = append(responseData, ToSoalResponse(soal))
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Soal berhasil diproses dan disimpan",
		"jenis":   jenis,
		"jumlah":  len(responseData),
		"data":    responseData,
	})
}

func (h *SoalHandler) GetSoalByModul(c *gin.Context) {
	// FIX: Safe type checking untuk kelas_id
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

	// FIX: Hapus logika r.Shuffle pada soal jawaban. Mapping langsung ke DTO.
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