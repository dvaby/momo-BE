package main

import (
	"momo-be/internal/config"
	"momo-be/internal/database"
	"momo-be/internal/handler"
	"momo-be/internal/repository"
	"momo-be/internal/router"
	"momo-be/internal/service"
	"momo-be/pkg/aiclient"
)

func main() {
	cfg := config.LoadConfig()
	db := database.Connect(cfg)

	aiClient := aiclient.NewClient(cfg.AIServiceURL)

	modulRepo := repository.NewModulRepository(db)
	modulService := service.NewModulService(modulRepo)
	modulHandler := handler.NewModulHandler(modulService)

	uploadHandler := handler.NewUploadHandler()

	materiRepo := repository.NewMateriRepository(db)
	materiService := service.NewMateriService(materiRepo, aiClient)
	materiHandler := handler.NewMateriHandler(materiService)

	soalRepo := repository.NewSoalRepository(db)
	soalService := service.NewSoalService(soalRepo, aiClient)
	soalHandler := handler.NewSoalHandler(soalService)

	kelasRepo := repository.NewKelasRepository(db)
	kelasService := service.NewKelasService(kelasRepo)
	kelasHandler := handler.NewKelasHandler(kelasService)

	siswaRepo := repository.NewSiswaRepository(db)
	siswaService := service.NewSiswaService(siswaRepo, kelasRepo)
	siswaHandler := handler.NewSiswaHandler(siswaService)

		r := router.SetupRouter(modulHandler, uploadHandler, materiHandler, soalHandler, kelasHandler, siswaHandler)
	r.Run(":" + cfg.ServerPort)
}