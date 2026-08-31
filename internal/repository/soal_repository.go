package repository

import (
	"gorm.io/gorm"

	"momo-be/internal/model"
)

type SoalRepository struct {
	db *gorm.DB
}

func NewSoalRepository(db *gorm.DB) *SoalRepository {
	return &SoalRepository{db: db}
}

func (r *SoalRepository) CreateBatch(soalList []model.Soal) error {
	if len(soalList) == 0 {
		return nil
	}
	return r.db.Create(&soalList).Error
}

func (r *SoalRepository) FindByModulAndJenis(modulID uint, jenis model.JenisSoal) ([]model.Soal, error) {
	var soalList []model.Soal
	err := r.db.Where("modul_id = ? AND jenis = ?", modulID, jenis).Find(&soalList).Error
	return soalList, err
}

// FindByID mengambil 1 soal lengkap (termasuk kunci_jawaban) berdasarkan
// ID-nya. Dipakai internal oleh JawabanSiswaService untuk menyusun
// request evaluasi ke AI Service — TIDAK boleh dipakai untuk endpoint
// yang diakses langsung oleh siswa, karena ini membawa kunci_jawaban.
func (r *SoalRepository) FindByID(id uint) (*model.Soal, error) {
	var soal model.Soal
	err := r.db.First(&soal, id).Error
	if err != nil {
		return nil, err
	}
	return &soal, nil
}