package handler

import (
	"io"
	"log"
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

	go func() {
		defer os.Remove(tempFilePath)

		materiList, err := h.service.ProcessAndSaveMateri(uint(modulID), tempFilePath)
		if err != nil {
			log.Printf("[background] gagal memproses materi untuk modul %d: %v", modulID, err)
			return
		}
		log.Printf("[background] berhasil memproses %d materi untuk modul %d", len(materiList), modulID)
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"message": "PDF sedang diproses di background, cek detail modul beberapa saat lagi",
	})
}