package handler

import (
	"io"
	"log"
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

	// Validasi kepemilikan Modul dilakukan SEKARANG (synchronous), bukan di background,
	// supaya guru langsung tahu kalau ditolak, tanpa nunggu proses PDF/AI selesai dulu.
	if err := h.service.ValidateModulOwnership(uint(modulID), guruID); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
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

	_, err = io.Copy(tempFile, src)
	tempFile.Close() // tutup sekarang juga, bukan lewat defer, karena goroutine di bawah butuh path-nya, bukan handle Go yang sama
	if err != nil {
		os.Remove(tempFile.Name())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan file sementara"})
		return
	}

	tempFilePath := tempFile.Name()

	// Proses berat (ekstrak PDF + panggil AI Service + simpan) dikerjakan di background,
	// TIDAK menahan response HTTP ini. Guru/FE polling GET /modul/:id/soal untuk lihat hasilnya nanti.
	go func() {
		defer os.Remove(tempFilePath)

		soalList, err := h.service.ProcessAndSaveSoal(uint(modulID), jenis, tempFilePath, guruID)
		if err != nil {
			log.Printf("[background] gagal memproses soal untuk modul %d: %v", modulID, err)
			return
		}
		log.Printf("[background] berhasil memproses %d soal untuk modul %d", len(soalList), modulID)
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"message": "PDF sedang diproses di background, cek daftar soal beberapa saat lagi",
		"jenis":   jenis,
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