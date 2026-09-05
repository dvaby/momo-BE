package main

import (
	"momo-be/internal/config"
	"momo-be/internal/database"
	"momo-be/internal/handler"
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
	emailClient := emailsender.NewClient(cfg.ResendAPIKey)

	guruRepo := repository.NewGuruRepository(db)
	guruService := service.NewGuruService(guruRepo, emailClient, cfg.AppBaseURL)
	guruHandler := handler.NewGuruHandler(guruService)

	modulRepo := repository.NewModulRepository(db)
	modulService := service.NewModulService(modulRepo)
	modulHandler := handler.NewModulHandler(modulService)

	uploadHandler := handler.NewUploadHandler()

	materiRepo := repository.NewMateriRepository(db)
	materiService := service.NewMateriService(materiRepo, modulRepo, aiClient)
	materiHandler := handler.NewMateriHandler(materiService)

	kelasRepo := repository.NewKelasRepository(db)
	kelasService := service.NewKelasService(kelasRepo, modulRepo)
	kelasHandler := handler.NewKelasHandler(kelasService)

	soalRepo := repository.NewSoalRepository(db)
	soalService := service.NewSoalService(soalRepo, kelasRepo, modulRepo, aiClient)
	soalHandler := handler.NewSoalHandler(soalService)

	siswaRepo := repository.NewSiswaRepository(db)
	siswaService := service.NewSiswaService(siswaRepo, kelasRepo)
	siswaHandler := handler.NewSiswaHandler(siswaService)

	jawabanSiswaRepo := repository.NewJawabanSiswaRepository(db)
	jawabanSiswaService := service.NewJawabanSiswaService(jawabanSiswaRepo, soalRepo, siswaRepo, kelasRepo, aiClient)
	jawabanSiswaHandler := handler.NewJawabanSiswaHandler(jawabanSiswaService)

	nilaiRepo := repository.NewNilaiRepository(db)
	nilaiService := service.NewNilaiService(nilaiRepo, kelasRepo, modulRepo)
	nilaiHandler := handler.NewNilaiHandler(nilaiService)

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
	)
	r.Run(":" + cfg.ServerPort)
}