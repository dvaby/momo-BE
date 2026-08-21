package main

import (
	"momo-be/internal/config"
	"momo-be/internal/database"
	"momo-be/internal/handler"
	"momo-be/internal/repository"
	"momo-be/internal/router"
	"momo-be/internal/service"
)

func main() {
	cfg := config.LoadConfig()
	db := database.Connect(cfg)

	modulRepo := repository.NewModulRepository(db)
	modulService := service.NewModulService(modulRepo)
	modulHandler := handler.NewModulHandler(modulService)

	uploadHandler := handler.NewUploadHandler()

	r := router.SetupRouter(modulHandler, uploadHandler)
	r.Run(":" + cfg.ServerPort)
}