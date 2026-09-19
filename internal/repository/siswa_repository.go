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
// FindBySessionID mencari siswa berdasarkan session ID (auth tanpa token).
func (r *SiswaRepository) FindBySessionID(sessionID string) (*model.Siswa, error) {
	var siswa model.Siswa
	err := r.db.Where("session_id = ? AND session_id <> ''", sessionID).First(&siswa).Error
	if err != nil {
		return nil, err
	}
	return &siswa, nil
}

// SetSessionID mengikat session ID percakapan ke satu siswa.
func (r *SiswaRepository) SetSessionID(siswaID uint, sessionID string) error {
	return r.db.Model(&model.Siswa{}).Where("id = ?", siswaID).Update("session_id", sessionID).Error
}
