package model

import "time"

type Soal struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	ModulID      uint      `gorm:"not null" json:"modul_id"`
	Jenis        JenisSoal `gorm:"type:varchar(20);not null;default:'harian'" json:"jenis"`
	Pertanyaan   string    `gorm:"type:text;not null" json:"pertanyaan"`
	PilihanA     string    `gorm:"type:text" json:"pilihan_a"`
	PilihanB     string    `gorm:"type:text" json:"pilihan_b"`
	PilihanC     string    `gorm:"type:text" json:"pilihan_c"`
	PilihanD     string    `gorm:"type:text" json:"pilihan_d"`
	KunciJawaban string    `gorm:"type:varchar(1)" json:"kunci_jawaban"`
	CreatedAt    time.Time `json:"created_at"`
}