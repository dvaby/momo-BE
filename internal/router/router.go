package router

import (
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"momo-be/internal/config"
	"momo-be/internal/handler"
	"momo-be/internal/middleware"
)

func SetupRouter(
	cfg *config.Config,
	modulHandler *handler.ModulHandler,
	uploadHandler *handler.UploadHandler,
	materiHandler *handler.MateriHandler,
	soalHandler *handler.SoalHandler,
	kelasHandler *handler.KelasHandler,
	siswaHandler *handler.SiswaHandler,
	jawabanSiswaHandler *handler.JawabanSiswaHandler,
	nilaiHandler *handler.NilaiHandler,
	guruHandler *handler.GuruHandler,
	aiCallbackHandler *handler.AICallbackHandler,
	streamHandler *handler.StreamHandler,
	tutorHandler *handler.TutorHandler,
) *gin.Engine {
	r := gin.Default()

	authLimiter := middleware.NewIPRateLimiter(rate.Every(12*time.Second), 5)
	aiLimiter := middleware.NewIPRateLimiter(rate.Every(6*time.Second), 3)

	rawOrigins := strings.Split(cfg.CORSAllowedOrigins, ",")
	var allowedStatic []string
	for _, o := range rawOrigins {
		trimmed := strings.TrimSpace(o)
		if trimmed != "" {
			allowedStatic = append(allowedStatic, trimmed)
		}
	}

	r.Use(cors.New(cors.Config{
		AllowOriginFunc: func(origin string) bool {
			for _, o := range allowedStatic {
				if origin == o {
					return true
				}
			}
			return strings.HasSuffix(origin, ".vercel.app") || origin == "https://vercel.app"
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "message": "server is running!"})
	})

	api := r.Group("/api/v1")
	{
		// ==================== PUBLIC ENDPOINTS ====================
		api.POST("/guru/register", middleware.RateLimiterMiddleware(authLimiter), guruHandler.Register)
		api.POST("/guru/login", middleware.RateLimiterMiddleware(authLimiter), guruHandler.Login)
		api.GET("/guru/verify-email", guruHandler.VerifyEmail)

		api.POST("/join", middleware.RateLimiterMiddleware(authLimiter), siswaHandler.JoinSiswa)
		api.POST("/test-extract-pdf", middleware.RateLimiterMiddleware(aiLimiter), uploadHandler.TestExtractPDF)

		// ==================== SISWA ENDPOINTS ====================
		siswaAuth := api.Group("")
siswaAuth.Use(middleware.AuthMiddleware())
{
	siswaAuth.GET("/modul/:id/soal", soalHandler.GetSoalByModul)
	siswaAuth.POST("/submit-jawaban", middleware.RateLimiterMiddleware(aiLimiter), jawabanSiswaHandler.SubmitJawaban)
	siswaAuth.GET("/siswa/kelas-saya", siswaHandler.GetKelasSaya)
	siswaAuth.GET("/siswa/modul/:id", siswaHandler.GetModulDetailForSiswa)
	siswaAuth.GET("/siswa/modul/:id/materi", siswaHandler.GetMateriForSiswa)
	siswaAuth.GET("/siswa/modul/:id/soal", soalHandler.GetSoalByModul)
	siswaAuth.POST("/tutor", middleware.RateLimiterMiddleware(aiLimiter), tutorHandler.SubmitTutor)
	siswaAuth.POST("/chat", middleware.RateLimiterMiddleware(aiLimiter), tutorHandler.SubmitTutor)
}

		// ==================== GURU ENDPOINTS ====================
		guruAuth := api.Group("")
		guruAuth.Use(middleware.GuruAuthMiddleware())
		{
			// Modul
			guruAuth.GET("/modul", modulHandler.GetAllModuls)
			guruAuth.GET("/modul/:id", modulHandler.GetModulByID)
			guruAuth.POST("/modul", modulHandler.CreateModul)
			guruAuth.PUT("/modul/:id", modulHandler.UpdateModul)
			guruAuth.DELETE("/modul/:id", modulHandler.DeleteModul)

			// Materi
			guruAuth.POST("/modul/:id/materi", materiHandler.UploadMateri)
			guruAuth.GET("/modul/:id/materi", materiHandler.ListMateri)
			guruAuth.POST("/modul/:id/materi/manual", materiHandler.CreateMateriManual)
			guruAuth.PUT("/materi/:id", materiHandler.UpdateMateri)
			guruAuth.DELETE("/materi/:id", materiHandler.DeleteMateri)

			// Soal
			guruAuth.POST("/modul/:id/soal", soalHandler.UploadSoal)
			guruAuth.GET("/modul/:id/soal/list", soalHandler.GetSoalByModulForGuru)
			guruAuth.POST("/modul/:id/soal/manual", soalHandler.CreateSoalManual)
			guruAuth.PUT("/soal/:id", soalHandler.UpdateSoal)
			guruAuth.DELETE("/soal/:id", soalHandler.DeleteSoal)

			// Kelas
			guruAuth.POST("/kelas", kelasHandler.CreateKelas)
			guruAuth.GET("/kelas", kelasHandler.GetKelasGuru)
			guruAuth.GET("/kelas/:id", kelasHandler.GetKelasByID)
			guruAuth.PUT("/kelas/:id", kelasHandler.UpdateKelas)
			guruAuth.DELETE("/kelas/:id", kelasHandler.DeleteKelas)

			guruAuth.POST("/kelas/:id/siswa", siswaHandler.DaftarkanSiswa)
			guruAuth.POST("/kelas/:id/modul", kelasHandler.AssignModul)
			guruAuth.DELETE("/kelas/:id/modul/:modul_id", kelasHandler.RemoveModul)

			guruAuth.GET("/kelas/:id/nilai", nilaiHandler.GetRekapNilai)

			// SSE stream lama (backward compatible)
			guruAuth.GET("/kelas/stream", kelasHandler.StreamKelas)
		}

		// ==================== INTERNAL ENDPOINTS ====================
		// TIDAK PERLU JWT — auth via query param ?token= di handler
		api.POST("/internal/ai-callback", aiCallbackHandler.Handle)

		// ==================== UNIFIED STREAM (FASE 2) ====================
		streamAuth := api.Group("")
		streamAuth.Use(middleware.UnifiedAuthMiddleware())
		{
			streamAuth.GET("/stream", streamHandler.HandleStream)
		}
	}

	return r
}