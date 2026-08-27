package repository

import (
	"momo-be/internal/model"

	"gorm.io/gorm"
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

func (r *KelasRepository) FindByID(id uint) (*model.Kelas, error) {
	var kelas model.Kelas
	err := r.db.Preload("Siswa").Preload("Modul").First(&kelas, id).Error
	if err != nil {
		return nil, err
	}
	return &kelas, nil
}

func (r *KelasRepository) FindByIDAndGuruID(id uint, guruID uint) (*model.Kelas, error) {
	var kelas model.Kelas
	err := r.db.Preload("Siswa").Preload("Modul").Where("id = ? AND guru_id = ?", id, guruID).First(&kelas).Error
	if err != nil {
		return nil, err
	}
	return &kelas, nil
}

func (r *KelasRepository) FindByKode(kode string) (*model.Kelas, error) {
	var kelas model.Kelas
	err := r.db.Where("kode = ?", kode).First(&kelas).Error
	if err != nil {
		return nil, err
	}
	return &kelas, nil
}

func (r *KelasRepository) IsKodeExists(kode string) (bool, error) {
	var count int64
	err := r.db.Model(&model.Kelas{}).Where("kode = ?", kode).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *KelasRepository) AssignModul(kelas *model.Kelas, modul *model.Modul) error {
	return r.db.Model(kelas).Association("Modul").Append(modul)
}

func (r *KelasRepository) IsModulInKelas(kelasID uint, modulID uint) (bool, error) {
	var count int64
	err := r.db.Table("kelas_moduls").
		Where("kelas_id = ? AND modul_id = ?", kelasID, modulID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}