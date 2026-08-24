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