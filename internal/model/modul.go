package model

import "time"

type Modul struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	GuruID    uint      `gorm:"not null" json:"guru_id"`
	Judul     string    `gorm:"type:varchar(255);not null" json:"judul"`
	Deskripsi string    `gorm:"type:text" json:"deskripsi"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relasi
	Materi []Materi `gorm:"foreignKey:ModulID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"materi"`
	Soal   []Soal   `gorm:"foreignKey:ModulID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"soal"`
	Kelas  []Kelas  `gorm:"many2many:kelas_moduls;" json:"kelas,omitempty"`
}