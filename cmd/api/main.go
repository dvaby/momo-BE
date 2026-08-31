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

	// Instansiasi Guru
	guruRepo := repository.NewGuruRepository(db)
	guruService := service.NewGuruService(guruRepo)
	guruHandler := handler.NewGuruHandler(guruService)

	// Instansiasi Modul
	modulRepo := repository.NewModulRepository(db)
	modulService := service.NewModulService(modulRepo)
	modulHandler := handler.NewModulHandler(modulService)

	uploadHandler := handler.NewUploadHandler()

	// Instansiasi Materi
	materiRepo := repository.NewMateriRepository(db)
	materiService := service.NewMateriService(materiRepo, aiClient)
	materiHandler := handler.NewMateriHandler(materiService)

	// Instansiasi Kelas
	kelasRepo := repository.NewKelasRepository(db)
	kelasService := service.NewKelasService(kelasRepo, modulRepo)
	kelasHandler := handler.NewKelasHandler(kelasService)

	// Instansiasi Soal
	soalRepo := repository.NewSoalRepository(db)
	soalService := service.NewSoalService(soalRepo, kelasRepo, modulRepo, aiClient)
	soalHandler := handler.NewSoalHandler(soalService)

	// Instansiasi Siswa
	siswaRepo := repository.NewSiswaRepository(db)
	siswaService := service.NewSiswaService(siswaRepo, kelasRepo)
	siswaHandler := handler.NewSiswaHandler(siswaService)

	// Instansiasi Jawaban Siswa
	jawabanSiswaRepo := repository.NewJawabanSiswaRepository(db)
	jawabanSiswaService := service.NewJawabanSiswaService(jawabanSiswaRepo, soalRepo, siswaRepo, kelasRepo, aiClient)
	jawabanSiswaHandler := handler.NewJawabanSiswaHandler(jawabanSiswaService)

	// Instansiasi Nilai (tambahkan modulRepo sebagai argumen ke-3)
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