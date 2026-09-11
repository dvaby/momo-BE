package model

import "time"

type Soal struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	ModulID      uint      `gorm:"not null;index;index:idx_soal_modul_jenis,priority:1" json:"modul_id"`
	Jenis        JenisSoal `gorm:"type:varchar(20);not null;index:idx_soal_modul_jenis,priority:2" json:"jenis"`
	Pertanyaan   string    `gorm:"type:text;not null" json:"pertanyaan"`
	PilihanA     string    `gorm:"type:text;not null" json:"pilihan_a"`
	PilihanB     string    `gorm:"type:text;not null" json:"pilihan_b"`
	PilihanC     string    `gorm:"type:text;not null" json:"pilihan_c"`
	PilihanD     string    `gorm:"type:text;not null" json:"pilihan_d"`
	KunciJawaban string    `gorm:"type:varchar(1);not null" json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
