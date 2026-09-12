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

func (r *MateriRepository) CreateBatch(materiList []model.Materi) error {
	if len(materiList) == 0 {
		return nil
	}
	return r.db.Create(&materiList).Error
}

// Create menyimpan satu materi baru
func (r *MateriRepository) Create(materi *model.Materi) error {
	return r.db.Create(materi).Error
}

// FindByModulID mengambil semua materi dalam satu modul, urut berdasarkan urutan
func (r *MateriRepository) FindByModulID(modulID uint) ([]model.Materi, error) {
	var materiList []model.Materi
	err := r.db.Where("modul_id = ?", modulID).Order("urutan ASC").Find(&materiList).Error
	return materiList, err
}

// FindByID mengambil satu materi berdasarkan ID
func (r *MateriRepository) FindByID(id uint) (*model.Materi, error) {
	var materi model.Materi
	err := r.db.First(&materi, id).Error
	return &materi, err
}

// Update menyimpan perubahan materi
func (r *MateriRepository) Update(materi *model.Materi) error {
	return r.db.Save(materi).Error
}

// Delete menghapus satu materi
func (r *MateriRepository) Delete(id uint) error {
	return r.db.Delete(&model.Materi{}, id).Error
}
