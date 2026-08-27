package model

import "time"

type Kelas struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	GuruID    uint       `gorm:"not null" json:"guru_id"`
	NamaKelas string    `gorm:"type:varchar(255);not null" json:"nama_kelas"`
	KodeKelas string    `gorm:"type:varchar(10);unique;not null" json:"kode_kelas"`
	Siswa     []Siswa   `gorm:"foreignKey:KelasID" json:"siswa,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}