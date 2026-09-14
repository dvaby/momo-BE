package repository

import (
	"gorm.io/gorm"

	"momo-be/internal/model"
)

type SoalRepository struct {
	db *gorm.DB
}

func NewSoalRepository(db *gorm.DB) *SoalRepository {
	return &SoalRepository{db: db}
}

// CreateBatch menyimpan banyak soal sekaligus (dipakai oleh proses ekstraksi AI)
func (r *SoalRepository) CreateBatch(soalList []model.Soal) error {
	if len(soalList) == 0 {
		return nil
	}
	return r.db.Create(&soalList).Error
}

// Create menyimpan satu soal baru (dipakai oleh CRUD manual)
func (r *SoalRepository) Create(soal *model.Soal) error {
	return r.db.Create(soal).Error
}

// FindByModulAndJenis mengambil soal dalam satu modul.
// Jika `jenis` kosong (""), ambil semua jenis. Jika diisi, filter berdasarkan jenis.
// Hasil diurutkan berdasarkan `id` ASC.
func (r *SoalRepository) FindByModulAndJenis(modulID uint, jenis model.JenisSoal) ([]model.Soal, error) {
	var soalList []model.Soal
	query := r.db.Where("modul_id = ?", modulID)
	if jenis != "" {
		query = query.Where("jenis = ?", jenis)
	}
	err := query.Order("id ASC").Find(&soalList).Error
	return soalList, err
}

// FindByID mengambil 1 soal lengkap (termasuk kunci_jawaban) berdasarkan ID-nya.
// Dipakai internal oleh JawabanSiswaService untuk menyusun request evaluasi ke AI Service
// — TIDAK boleh dipakai untuk endpoint yang diakses langsung oleh siswa.
func (r *SoalRepository) FindByID(id uint) (*model.Soal, error) {
	var soal model.Soal
	err := r.db.First(&soal, id).Error
	if err != nil {
		return nil, err
	}
	return &soal, nil
}

// Update menyimpan perubahan soal
func (r *SoalRepository) Update(soal *model.Soal) error {
	return r.db.Save(soal).Error
}

// Delete menghapus satu soal
func (r *SoalRepository) Delete(id uint) error {
	return r.db.Delete(&model.Soal{}, id).Error
}