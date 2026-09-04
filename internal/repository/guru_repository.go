package repository

import (
	"momo-be/internal/model"

	"gorm.io/gorm"
)

type GuruRepository interface {
	Create(guru *model.Guru) error
	FindByEmail(email string) (*model.Guru, error)
	FindByID(id uint) (*model.Guru, error)
	FindByVerificationToken(token string) (*model.Guru, error)
	Update(guru *model.Guru) error
}

type guruRepository struct {
	db *gorm.DB
}

func NewGuruRepository(db *gorm.DB) GuruRepository {
	return &guruRepository{db: db}
}

func (r *guruRepository) Create(guru *model.Guru) error {
	return r.db.Create(guru).Error
}

func (r *guruRepository) FindByEmail(email string) (*model.Guru, error) {
	var guru model.Guru
	err := r.db.Where("email = ?", email).First(&guru).Error
	if err != nil {
		return nil, err
	}
	return &guru, nil
}

func (r *guruRepository) FindByID(id uint) (*model.Guru, error) {
	var guru model.Guru
	err := r.db.First(&guru, id).Error
	if err != nil {
		return nil, err
	}
	return &guru, nil
}

func (r *guruRepository) FindByVerificationToken(token string) (*model.Guru, error) {
	var guru model.Guru
	err := r.db.Where("verification_token = ?", token).First(&guru).Error
	if err != nil {
		return nil, err
	}
	return &guru, nil
}

func (r *guruRepository) Update(guru *model.Guru) error {
	return r.db.Save(guru).Error
}