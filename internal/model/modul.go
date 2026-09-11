package model

import "time"

type Modul struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	GuruID    uint      `gorm:"not null;index" json:"guru_id"`
	Nama      string    `gorm:"not null" json:"nama"`
	Deskripsi string    `json:"deskripsi"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relasi
	Materi []Materi `gorm:"foreignKey:ModulID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"materi"`
	Soal   []Soal   `gorm:"foreignKey:ModulID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"soal"`
	Kelas  []Kelas  `gorm:"many2many:kelas_moduls;" json:"kelas,omitempty"`
}
