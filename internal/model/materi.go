package model

import "time"

type Materi struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ModulID   uint      `gorm:"not null" json:"modul_id"`
	Urutan    int       `gorm:"not null" json:"urutan"`
	Judul     string    `gorm:"type:varchar(255)" json:"judul"`
	Konten    string    `gorm:"type:text;not null" json:"konten"`
	CreatedAt time.Time `json:"created_at"`
}