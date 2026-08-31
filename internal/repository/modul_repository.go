package repository

import (
	"momo-be/internal/model"

	"gorm.io/gorm"
)

type ModulRepository interface {
	Create(modul *model.Modul) error
	FindAll() ([]model.Modul, error)
	FindByID(id uint) (*model.Modul, error)
	FindByGuruID(guruID uint) ([]model.Modul, error)
	FindByIDAndGuruID(id uint, guruID uint) (*model.Modul, error)
	Update(modul *model.Modul) error
	Delete(id uint) error
}

type modulRepository struct {
	db *gorm.DB
}

func NewModulRepository(db *gorm.DB) ModulRepository {
	return &modulRepository{db: db}
}

func (r *modulRepository) Create(modul *model.Modul) error {
	return r.db.Create(modul).Error
}

func (r *modulRepository) FindAll() ([]model.Modul, error) {
	var moduls []model.Modul
	err := r.db.Find(&moduls).Error
	return moduls, err
}

func (r *modulRepository) FindByID(id uint) (*model.Modul, error) {
	var modul model.Modul
	err := r.db.Preload("Materi").Preload("Soal").First(&modul, id).Error
	if err != nil {
		return nil, err
	}
	return &modul, nil
}

func (r *modulRepository) FindByGuruID(guruID uint) ([]model.Modul, error) {
	var moduls []model.Modul
	err := r.db.Where("guru_id = ?", guruID).Find(&moduls).Error
	return moduls, err
}

func (r *modulRepository) FindByIDAndGuruID(id uint, guruID uint) (*model.Modul, error) {
	var modul model.Modul
	err := r.db.Where("id = ? AND guru_id = ?", id, guruID).First(&modul).Error
	if err != nil {
		return nil, err
	}
	return &modul, nil
}

func (r *modulRepository) Update(modul *model.Modul) error {
	return r.db.Save(modul).Error
}

func (r *modulRepository) Delete(id uint) error {
	return r.db.Delete(&model.Modul{}, id).Error
}