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