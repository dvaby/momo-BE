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