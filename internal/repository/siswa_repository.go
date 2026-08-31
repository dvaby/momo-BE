package repository

import (
	"momo-be/internal/model"

	"gorm.io/gorm"
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

func (r *SiswaRepository) FindByID(id uint) (*model.Siswa, error) {
	var siswa model.Siswa
	err := r.db.First(&siswa, id).Error
	if err != nil {
		return nil, err
	}
	return &siswa, nil
}

func (r *SiswaRepository) FindByNamaAndKelasID(nama string, kelasID uint) (*model.Siswa, error) {
	var siswa model.Siswa
	err := r.db.Where("nama = ? AND kelas_id = ?", nama, kelasID).First(&siswa).Error
	if err != nil {
		return nil, err
	}
	return &siswa, nil
}

func (r *SiswaRepository) FindByKelasID(kelasID uint) ([]model.Siswa, error) {
	var siswas []model.Siswa
	err := r.db.Where("kelas_id = ?", kelasID).Find(&siswas).Error
	if err != nil {
		return nil, err
	}
	return siswas, nil
}