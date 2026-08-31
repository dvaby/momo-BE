package handler

import (
	"io"
	"net/http"
	"os"
	"strconv"

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
	modulIDParam := c.Param("id")
	modulID, err := strconv.ParseUint(modulIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID modul tidak valid"})
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

	tempFile, err := os.CreateTemp("", "materi-*.pdf")
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

	materiList, err := h.service.ProcessAndSaveMateri(uint(modulID), tempFile.Name())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Materi berhasil diproses dan disimpan",
		"jumlah":  len(materiList),
		"data":    materiList,
	})
}