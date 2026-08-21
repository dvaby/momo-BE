package repository

import (
	"gorm.io/gorm"

	"momo-be/internal/model"
)

type ModulRepository struct {
	db *gorm.DB
}

func NewModulRepository(db *gorm.DB) *ModulRepository {
	return &ModulRepository{db: db}
}

func (r *ModulRepository) Create(modul *model.Modul) error {
	return r.db.Create(modul).Error
}

func (r *ModulRepository) FindAll() ([]model.Modul, error) {
	var modulList []model.Modul
	err := r.db.Find(&modulList).Error
	return modulList, err
}

func (r *ModulRepository) FindByID(id uint) (*model.Modul, error) {
	var modul model.Modul
	err := r.db.Preload("Materi").Preload("Soal").First(&modul, id).Error
	if err != nil {
		return nil, err
	}
	return &modul, nil
}