package model

import "time"

type Siswa struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	KelasID   uint      `gorm:"not null" json:"kelas_id"`
	Nama      string    `gorm:"type:varchar(255);not null" json:"nama"`
	CreatedAt time.Time `json:"created_at"`
}