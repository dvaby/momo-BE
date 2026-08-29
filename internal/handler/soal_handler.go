package handler

import (
	"io"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"

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

	soalList, err := h.service.ProcessAndSaveSoal(uint(modulID), jenis, tempFile.Name())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Soal berhasil diproses dan disimpan",
		"jenis":   jenis,
		"jumlah":  len(soalList),
		"data":    soalList,
	})
}

func (h *SoalHandler) GetSoalByModul(c *gin.Context) {
	kelasIDVal, exists := c.Get("kelas_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Akses khusus siswa"})
		return
	}
	kelasID := kelasIDVal.(uint)

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

	// Inisialisasi seed acak berdasarkan waktu saat ini
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	responseData := make([]SoalSiswaResponse, 0, len(soalList))
	for _, soal := range soalList {
		resp := ToSoalSiswaResponse(soal)

		// Kumpulkan pilihan jawaban ke dalam slice
		pilihan := []string{resp.PilihanA, resp.PilihanB, resp.PilihanC, resp.PilihanD}

		// Acak urutan pilihan jawaban
		r.Shuffle(len(pilihan), func(i, j int) {
			pilihan[i], pilihan[j] = pilihan[j], pilihan[i]
		})

		// Re-assign pilihan yang sudah diacak
		resp.PilihanA = pilihan[0]
		resp.PilihanB = pilihan[1]
		resp.PilihanC = pilihan[2]
		resp.PilihanD = pilihan[3]

		responseData = append(responseData, resp)
	}

	c.JSON(http.StatusOK, gin.H{
		"jenis":  jenis,
		"jumlah": len(responseData),
		"data":   responseData,
	})
}