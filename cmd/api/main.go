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
	"momo-be/pkg/aiclient"
	"momo-be/pkg/emailsender"
)

func main() {
	cfg := config.LoadConfig()
	db := database.Connect(cfg)

	aiClient := aiclient.NewClient(cfg.AIServiceURL)
	sseHub := handler.NewSSEHub()

	// BARU Fase 1: Job registry untuk arsitektur v1.4+
	jobRegistry := job.NewRegistry()

	// Cleanup job lama setiap 5 menit (retention 1 jam) — mencegah memory leak
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
	materiService := service.NewMateriService(materiRepo, modulRepo, aiClient)
	materiHandler := handler.NewMateriHandler(materiService)

	kelasRepo := repository.NewKelasRepository(db)
	kelasService := service.NewKelasService(kelasRepo, modulRepo)
	kelasHandler := handler.NewKelasHandler(kelasService, sseHub)

	soalRepo := repository.NewSoalRepository(db)
	soalService := service.NewSoalService(soalRepo, kelasRepo, modulRepo, aiClient)
	soalHandler := handler.NewSoalHandler(soalService)

	siswaRepo := repository.NewSiswaRepository(db)
	siswaService := service.NewSiswaService(siswaRepo, kelasRepo, materiRepo, soalRepo)
	siswaHandler := handler.NewSiswaHandler(siswaService)

	jawabanSiswaRepo := repository.NewJawabanSiswaRepository(db)
	jawabanSiswaService := service.NewJawabanSiswaService(jawabanSiswaRepo, soalRepo, siswaRepo, kelasRepo, aiClient)
	jawabanSiswaHandler := handler.NewJawabanSiswaHandler(jawabanSiswaService)

	nilaiRepo := repository.NewNilaiRepository(db)
	nilaiService := service.NewNilaiService(nilaiRepo, kelasRepo, modulRepo)
	nilaiHandler := handler.NewNilaiHandler(nilaiService)

	// BARU Fase 1: handler callback AI
	aiCallbackHandler := handler.NewAICallbackHandler(jobRegistry, cfg.AIInternalToken)

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
		aiCallbackHandler, // BARU
	)
	r.Run(":" + cfg.ServerPort)
}
