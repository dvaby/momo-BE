package database

import (
	"fmt"
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"momo-be/internal/config"
	"momo-be/internal/model"
)

func Connect(cfg *config.Config) *gorm.DB {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=Asia/Jakarta",
		cfg.DBHost, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBPort, cfg.DBSSLMode,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		// Hanya log warning + slow query, biar log Railway tidak berisik
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("Gagal terhubung ke database: %v", err)
	}

	// ---- CONNECTION POOLING (penting untuk serverless DB) ----
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Gagal mengambil underlying sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(10)                  // maksimal 10 koneksi simultan
	sqlDB.SetMaxIdleConns(5)                   // simpan 5 koneksi menganggur siap pakai
	sqlDB.SetConnMaxLifetime(30 * time.Minute) // daur ulang koneksi tiap 30 menit
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)  // buang koneksi idle > 5 menit

	// Auto Migration Table (sekaligus membuat index dari tag model)
	err = db.AutoMigrate(
		&model.Guru{},
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

	// ---- KEEP-WARM NEON ----
	// Ping DB tiap 4 menit agar compute Neon tidak "tidur" (scale-to-zero),
	// sehingga query pertama pengguna tidak kena cold start ~1 detik.
	go func() {
		ticker := time.NewTicker(4 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			if err := sqlDB.Ping(); err != nil {
				log.Printf("keep-warm ping gagal: %v", err)
			}
		}
	}()

	log.Println("Berhasil terhubung ke database dan auto migration selesai!")
	return db
}
