package model

import "time"

type Siswa struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	KelasID   uint      `gorm:"not null;index;index:idx_siswa_kelas_nama,priority:1" json:"kelas_id"`
	Nama      string    `gorm:"type:varchar(255);not null;index:idx_siswa_kelas_nama,priority:2" json:"nama"`
	CreatedAt time.Time `json:"created_at"`
}
