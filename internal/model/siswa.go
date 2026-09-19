package model

import "time"

type Siswa struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	KelasID   uint      `gorm:"not null" json:"kelas_id"`
	Nama      string    `gorm:"not null" json:"nama"`
	SessionID string    `gorm:"index" json:"-"` // identitas alur suara (pengganti token)
	CreatedAt time.Time `json:"created_at"`
}

