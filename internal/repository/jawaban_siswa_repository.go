package repository

import (
	"gorm.io/gorm"

	"momo-be/internal/model"
)

type JawabanSiswaRepository struct {
	db *gorm.DB
}

func NewJawabanSiswaRepository(db *gorm.DB) *JawabanSiswaRepository {
	return &JawabanSiswaRepository{db: db}
}

// Create menyimpan satu percobaan jawaban siswa (baris baru).
func (r *JawabanSiswaRepository) Create(jawaban *model.JawabanSiswa) error {
	return r.db.Create(jawaban).Error
}

// FindBySiswaIDAndSoalID dipakai untuk mengecek apakah siswa sudah
// pernah menjawab soal ini sebelumnya. Dipakai khusus untuk aturan
// "1x submit" pada soal jenis uts/uas.
func (r *JawabanSiswaRepository) FindBySiswaIDAndSoalID(siswaID uint, soalID uint) (*model.JawabanSiswa, error) {
	var jawaban model.JawabanSiswa
	err := r.db.Where("siswa_id = ? AND soal_id = ?", siswaID, soalID).First(&jawaban).Error
	if err != nil {
		return nil, err
	}
	return &jawaban, nil
}