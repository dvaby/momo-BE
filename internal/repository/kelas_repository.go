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
	return &kelas, err
}

func (r *KelasRepository) FindByIDAndGuruID(id, guruID uint) (*model.Kelas, error) {
	var kelas model.Kelas
	err := r.db.Preload("Siswa").Preload("Modul").First(&kelas, "id = ? AND guru_id = ?", id, guruID).Error
	return &kelas, err
}

func (r *KelasRepository) FindByGuruID(guruID uint) ([]model.Kelas, error) {
	var kelas []model.Kelas
	err := r.db.Preload("Siswa").Preload("Modul").Where("guru_id = ?", guruID).Find(&kelas).Error
	return kelas, err
}

func (r *KelasRepository) FindByKodeKelas(kode string) (*model.Kelas, error) {
	var kelas model.Kelas
	err := r.db.First(&kelas, "kode_kelas = ?", kode).Error
	return &kelas, err
}

func (r *KelasRepository) Update(kelas *model.Kelas) error {
	return r.db.Save(kelas).Error
}

func (r *KelasRepository) Delete(id uint) error {
	return r.db.Delete(&model.Kelas{}, id).Error
}

func (r *KelasRepository) IsKodeExists(kode string) (bool, error) {
	var count int64
	err := r.db.Model(&model.Kelas{}).Where("kode_kelas = ?", kode).Count(&count).Error
	return count > 0, err
}

func (r *KelasRepository) AssignModul(kelas *model.Kelas, modul *model.Modul) error {
	return r.db.Model(kelas).Association("Modul").Append(modul)
}

func (r *KelasRepository) RemoveModul(kelas *model.Kelas, modulID uint) error {
	var modul model.Modul
	if err := r.db.First(&modul, modulID).Error; err != nil {
		return err
	}
	return r.db.Model(kelas).Association("Modul").Delete(&modul)
}

// FindByKode mencari kelas berdasarkan kode_kelas
func (r *KelasRepository) FindByKode(kode string) (*model.Kelas, error) {
	var kelas model.Kelas
	err := r.db.Where("kode_kelas = ?", kode).First(&kelas).Error
	if err != nil {
		return nil, err
	}
	return &kelas, nil
}

// IsModulInKelas mengecek apakah modul sudah di-assign ke kelas tertentu
func (r *KelasRepository) IsModulInKelas(kelasID, modulID uint) (bool, error) {
	var count int64
	// 'kelas_moduls' adalah nama tabel many-to-many default dari GORM berdasarkan model kamu
	err := r.db.Table("kelas_moduls").Where("kelas_id = ? AND modul_id = ?", kelasID, modulID).Count(&count).Error
	return count > 0, err
}