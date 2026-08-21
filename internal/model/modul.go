package model

import "time"

type Modul struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Judul     string    `gorm:"type:varchar(255);not null" json:"judul"`
	Deskripsi string    `gorm:"type:text" json:"deskripsi"`
	Materi    []Materi  `gorm:"foreignKey:ModulID" json:"materi,omitempty"`
	Soal      []Soal    `gorm:"foreignKey:ModulID" json:"soal,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}