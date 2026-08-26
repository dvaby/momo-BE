package router

import (
	"github.com/gin-gonic/gin"

	"momo-be/internal/handler"
	"momo-be/internal/middleware"
)

func SetupRouter(
	modulHandler *handler.ModulHandler,
	uploadHandler *handler.UploadHandler,
	materiHandler *handler.MateriHandler,
	soalHandler *handler.SoalHandler,
	kelasHandler *handler.KelasHandler,
	siswaHandler *handler.SiswaHandler,
	jawabanSiswaHandler *handler.JawabanSiswaHandler,
	nilaiHandler *handler.NilaiHandler,
	guruHandler *handler.GuruHandler, // <--- TAMBAHAN: Inject GuruHandler
) *gin.Engine {
	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "message": "server is running!"})
	})

	api := r.Group("/api/v1")
	{
		// --- Public Routes Guru (Auth) ---
		api.POST("/guru/register", guruHandler.Register)
		api.POST("/guru/login", guruHandler.Login)

		// --- Public Routes Siswa & General ---
		api.POST("/join", siswaHandler.JoinSiswa)
		api.GET("/modul", modulHandler.GetAllModul)
		api.GET("/modul/:id", modulHandler.GetModulByID)
		api.POST("/test-extract-pdf", uploadHandler.TestExtractPDF)

		// --- Protected Routes Siswa (Wajib Token Siswa) ---
		siswaAuth := api.Group("")
		siswaAuth.Use(middleware.AuthMiddleware())
		{
			siswaAuth.GET("/modul/:id/soal", soalHandler.GetSoalByModul)
			siswaAuth.POST("/submit-jawaban", jawabanSiswaHandler.SubmitJawaban)
		}

		// --- Protected Routes Guru (Wajib Token Guru) ---
		guruAuth := api.Group("")
		guruAuth.Use(middleware.GuruAuthMiddleware())
		{
			guruAuth.POST("/modul", modulHandler.CreateModul)
			guruAuth.POST("/modul/:id/materi", materiHandler.UploadMateri)
			guruAuth.POST("/modul/:id/soal", soalHandler.UploadSoal)

			guruAuth.POST("/kelas", kelasHandler.CreateKelas)
			guruAuth.GET("/kelas/:id", kelasHandler.GetKelasByID)
			guruAuth.POST("/kelas/:id/siswa", siswaHandler.DaftarkanSiswa)

			guruAuth.GET("/kelas/:id/nilai", nilaiHandler.GetRekapNilai)
		}
	}

	return r
}