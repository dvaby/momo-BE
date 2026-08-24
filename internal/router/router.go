package router

import (
	"github.com/gin-gonic/gin"

	"momo-be/internal/handler"
)

func SetupRouter(modulHandler *handler.ModulHandler, uploadHandler *handler.UploadHandler, materiHandler *handler.MateriHandler) *gin.Engine {
	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "message": "server is running!"})
	})

	api := r.Group("/api/v1")
	{
		api.POST("/modul", modulHandler.CreateModul)
		api.GET("/modul", modulHandler.GetAllModul)
		api.GET("/modul/:id", modulHandler.GetModulByID)
		api.POST("/modul/:id/materi", materiHandler.UploadMateri)

		api.POST("/test-extract-pdf", uploadHandler.TestExtractPDF)
	}

	return r
}