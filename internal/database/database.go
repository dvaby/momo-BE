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
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Jakarta",
		cfg.DBHost, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBPort,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Gagal terhubung ke database: %v", err)
	}

	// Auto Migration Table
	err = db.AutoMigrate(
		&model.Guru{}, // <--- TAMBAHAN: AutoMigrate tabel gurus
		&model.Modul{},
		&model.Materi{},
		&model.Soal{},
		&model.Kelas{},
		&model.Siswa{},
		&model.JawabanSiswa{},
	)
	if err != nil {
		log.Fatalf("Gagal melakukan auto migration: %v", err)
	}

	log.Println("Berhasil terhubung ke database dan auto migration selesai!")
	return db
}