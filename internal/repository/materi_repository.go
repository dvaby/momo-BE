package repository

import (
	"gorm.io/gorm"

	"momo-be/internal/model"
)

type MateriRepository struct {
	db *gorm.DB
}

func NewMateriRepository(db *gorm.DB) *MateriRepository {
	return &MateriRepository{db: db}
}

// Create menyimpan materi baru
func (r *MateriRepository) Create(materi *model.Materi) error {
	return r.db.Create(materi).Error
}

// CreateBatch menyimpan banyak materi sekaligus (bulk insert)
func (r *MateriRepository) CreateBatch(materis []model.Materi) error {
	if len(materis) == 0 {
		return nil
	}
	return r.db.Create(&materis).Error
}

// FindByModulID mengambil semua materi dalam satu modul
func (r *MateriRepository) FindByModulID(modulID uint) ([]model.Materi, error) {
	var materis []model.Materi
	err := r.db.Where("modul_id = ?", modulID).Find(&materis).Error
	return materis, err
}

// FindByID mengambil satu materi berdasarkan ID
func (r *MateriRepository) FindByID(id uint) (*model.Materi, error) {
	var materi model.Materi
	if err := r.db.First(&materi, id).Error; err != nil {
		return nil, err
	}
	return &materi, nil
}

// GetModulByMateriID mengambil modul induk dari sebuah materi (query manual via modul_id)
func (r *MateriRepository) GetModulByMateriID(materiID uint) (*model.Modul, error) {
	var materi model.Materi
	if err := r.db.First(&materi, materiID).Error; err != nil {
		return nil, err
	}

	var modul model.Modul
	if err := r.db.First(&modul, materi.ModulID).Error; err != nil {
		return nil, err
	}

	return &modul, nil
}

// Update memperbarui materi
func (r *MateriRepository) Update(materi *model.Materi) error {
	return r.db.Save(materi).Error
}

// Delete menghapus materi
func (r *MateriRepository) Delete(id uint) error {
	return r.db.Delete(&model.Materi{}, id).Error
}