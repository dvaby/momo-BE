package router

import (
	"github.com/gin-gonic/gin"

	"momo-be/internal/handler"
	"momo-be/internal/middleware"
)

func SetupRouter(modulHandler *handler.ModulHandler, uploadHandler *handler.UploadHandler, materiHandler *handler.MateriHandler, soalHandler *handler.SoalHandler, kelasHandler *handler.KelasHandler, siswaHandler *handler.SiswaHandler, jawabanSiswaHandler *handler.JawabanSiswaHandler) *gin.Engine {
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
		api.POST("/modul/:id/soal", soalHandler.UploadSoal)
		api.GET("/modul/:id/soal", middleware.AuthMiddleware(), soalHandler.GetSoalByModul)
		api.POST("/kelas/:id/siswa", siswaHandler.DaftarkanSiswa)
		api.POST("/join", siswaHandler.JoinSiswa)
		api.POST("/submit-jawaban", middleware.AuthMiddleware(), jawabanSiswaHandler.SubmitJawaban)

		api.POST("/kelas", kelasHandler.CreateKelas)
		api.GET("/kelas/:id", kelasHandler.GetKelasByID)

		api.POST("/test-extract-pdf", uploadHandler.TestExtractPDF)
	}

	return r
}