package repository

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"

	"momo-be/internal/model"
)

type ProgressRepository struct{ db *gorm.DB }

func NewProgressRepository(db *gorm.DB) *ProgressRepository {
	return &ProgressRepository{db: db}
}

// Touch memperbarui timestamp aktivitas terakhir siswa
func (r *ProgressRepository) Touch(siswaID uint) {
	if siswaID == 0 {
		return
	}
	var p model.SiswaProgress
	if err := r.db.Where("siswa_id = ?", siswaID).First(&p).Error; err != nil {
		r.db.Create(&model.SiswaProgress{SiswaID: siswaID, MateriSelesai: "[]", LastActivity: time.Now()})
		return
	}
	r.db.Model(&p).Update("last_activity", time.Now())
}

// SelesaikanMateri menandai satu materi selesai untuk siswa
func (r *ProgressRepository) SelesaikanMateri(siswaID, materiID uint) {
	if siswaID == 0 || materiID == 0 {
		return
	}
	var p model.SiswaProgress
	if err := r.db.Where("siswa_id = ?", siswaID).First(&p).Error; err != nil {
		p = model.SiswaProgress{SiswaID: siswaID, MateriSelesai: "[]"}
	}
	var ids []uint
	_ = json.Unmarshal([]byte(p.MateriSelesai), &ids)
	sudah := false
	for _, id := range ids {
		if id == materiID {
			sudah = true
			break
		}
	}
	if !sudah {
		ids = append(ids, materiID)
	}
	b, _ := json.Marshal(ids)
	p.MateriSelesai = string(b)
	p.LastActivity = time.Now()
	r.db.Save(&p)
}