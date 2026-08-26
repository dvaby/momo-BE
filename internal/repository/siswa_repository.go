package repository

import (
	"gorm.io/gorm"

	"momo-be/internal/model"
)

type SiswaRepository struct {
	db *gorm.DB
}

func NewSiswaRepository(db *gorm.DB) *SiswaRepository {
	return &SiswaRepository{db: db}
}

func (r *SiswaRepository) Create(siswa *model.Siswa) error {
	return r.db.Create(siswa).Error
}

func (r *SiswaRepository) FindByNamaAndKelasID(nama string, kelasID uint) (*model.Siswa, error) {
	var siswa model.Siswa
	err := r.db.Where("nama = ? AND kelas_id = ?", nama, kelasID).First(&siswa).Error
	if err != nil {
		return nil, err
	}
	return &siswa, nil
}