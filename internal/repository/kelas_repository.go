package repository

import (
	"gorm.io/gorm"

	"momo-be/internal/model"
)

type KelasRepository struct {
	db *gorm.DB
}

func NewKelasRepository(db *gorm.DB) *KelasRepository {
	return &KelasRepository{db: db}
}

func (r *KelasRepository) Create(kelas *model.Kelas) error {
	return r.db.Create(kelas).Error
}

func (r *KelasRepository) IsKodeExists(kode string) (bool, error) {
	var count int64
	err := r.db.Model(&model.Kelas{}).Where("kode_kelas = ?", kode).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *KelasRepository) FindByKode(kode string) (*model.Kelas, error) {
	var kelas model.Kelas
	err := r.db.Where("kode_kelas = ?", kode).First(&kelas).Error
	if err != nil {
		return nil, err
	}
	return &kelas, nil
}

func (r *KelasRepository) FindByID(id uint) (*model.Kelas, error) {
	var kelas model.Kelas
	err := r.db.Preload("Siswa").First(&kelas, id).Error
	if err != nil {
		return nil, err
	}
	return &kelas, nil
}