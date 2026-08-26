package database

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"momo-be/internal/config"
	"momo-be/internal/model"
)

func Connect(cfg *config.Config) *gorm.DB {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Gagal konek ke database: %v", err)
	}
	log.Println("Berhasil konek ke database PostgreSQL!")

	err = db.AutoMigrate(&model.Modul{}, &model.Materi{}, &model.Soal{}, &model.Kelas{}, &model.Siswa{}, &model.JawabanSiswa{})
	if err != nil {
		log.Fatalf("Gagal melakukan migration: %v", err)
	}
	log.Println("Migration berhasil, tabel siap digunakan!")

	return db
}