package main

import (
	"log"
	"time"

	"momo-be/internal/config"
	"momo-be/internal/database"
	"momo-be/internal/handler"
	"momo-be/internal/job"
	"momo-be/internal/repository"
	"momo-be/internal/router"
	"momo-be/internal/service"
	"momo-be/internal/sse"
	"momo-be/pkg/aiclient"
	"momo-be/pkg/emailsender"
	"momo-be/internal/middleware"
)

func main() {
	cfg := config.LoadConfig()
	db := database.Connect(cfg)

	aiClient := aiclient.NewClient(cfg.AIServiceURL)
	sseHub := handler.NewSSEHub()

	// Job registry untuk arsitektur v1.4+
	jobRegistry := job.NewRegistry()

	// Cleanup job lama setiap 5 menit (retention 1 jam)
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			removed := jobRegistry.Cleanup(1 * time.Hour)
			if removed > 0 {
				log.Printf("[job-registry] cleaned up %d old jobs", removed)
			}
		}
	}()

	// Unified SSE hub untuk stream role-aware
	unifiedHub := sse.NewHub()
	streamHandler := handler.NewStreamHandler(unifiedHub)

	// Inisialisasi menggunakan Brevo, bukan SMTP
	emailClient := emailsender.NewClient(cfg.BrevoAPIKey, cfg.BrevoSenderEmail, cfg.BrevoSenderName)

	guruRepo := repository.NewGuruRepository(db)
	guruService := service.NewGuruService(guruRepo, emailClient, cfg.AppBaseURL)
	guruHandler := handler.NewGuruHandler(guruService, cfg.FEVerifyRedirectURL)

	modulRepo := repository.NewModulRepository(db)
	modulService := service.NewModulService(modulRepo)
	modulHandler := handler.NewModulHandler(modulService)

	uploadHandler := handler.NewUploadHandler()

	materiRepo := repository.NewMateriRepository(db)
	callbackURL := cfg.CallbackURLWithSecret()
	log.Printf("[startup] callback URL: %s...%s", callbackURL[:40], callbackURL[len(callbackURL)-10:])
	materiService := service.NewMateriService(materiRepo, modulRepo, aiClient, jobRegistry, callbackURL)
	materiHandler := handler.NewMateriHandler(materiService, unifiedHub)

	kelasRepo := repository.NewKelasRepository(db)
	kelasService := service.NewKelasService(kelasRepo, modulRepo)
	kelasHandler := handler.NewKelasHandler(kelasService, sseHub)

	soalRepo := repository.NewSoalRepository(db)
	soalService := service.NewSoalService(soalRepo, kelasRepo, modulRepo, aiClient, jobRegistry, callbackURL)
	soalHandler := handler.NewSoalHandler(soalService, unifiedHub)

	siswaRepo := repository.NewSiswaRepository(db)
	siswaService := service.NewSiswaService(siswaRepo, kelasRepo, materiRepo, soalRepo)
	siswaHandler := handler.NewSiswaHandler(siswaService)

	jawabanSiswaRepo := repository.NewJawabanSiswaRepository(db)
	jawabanSiswaService := service.NewJawabanSiswaService(jawabanSiswaRepo, soalRepo, siswaRepo, kelasRepo, aiClient, jobRegistry, callbackURL)
	jawabanSiswaHandler := handler.NewJawabanSiswaHandler(jawabanSiswaService, unifiedHub)

	nilaiRepo := repository.NewNilaiRepository(db)
nilaiService := service.NewNilaiService(nilaiRepo, kelasRepo, modulRepo)
nilaiHandler := handler.NewNilaiHandler(nilaiService)

// BARU: Tutor service untuk Mode Tutor Fase 2
toolsExecURL := cfg.ToolsExecURLWithSecret()
log.Printf("[startup] tools exec URL: %s...%s", toolsExecURL[:40], toolsExecURL[len(toolsExecURL)-10:])
tutorService := service.NewTutorService(aiClient, jobRegistry, callbackURL, toolsExecURL, siswaService)
toolsHandler := handler.NewToolsHandler(tutorService, cfg.AIInternalToken)
tutorHandler := handler.NewTutorHandler(tutorService)

// Handler callback AI (tambah unifiedHub untuk emit tutor-reply)
aiCallbackHandler := handler.NewAICallbackHandler(jobRegistry, cfg.AIInternalToken, unifiedHub)

	r := router.SetupRouter(
    cfg,
    modulHandler,
    uploadHandler,
    materiHandler,
    soalHandler,
    kelasHandler,
    siswaHandler,
    jawabanSiswaHandler,
    nilaiHandler,
    guruHandler,
    aiCallbackHandler,
    streamHandler,
    tutorHandler,
    middleware.NewAuthMiddleware(siswaRepo),
)
// TAMBAHKAN BARIS INI:
r.POST("/api/v1/internal/tools/execute", toolsHandler.Execute)
r.Run(":" + cfg.ServerPort)
}
