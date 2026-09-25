package model

import "time"

type SiswaProgress struct {
	ID            uint      `gorm:"primaryKey"`
	SiswaID       uint      `gorm:"uniqueIndex"`
	MateriSelesai string    `gorm:"type:text;default:'[]'"` // JSON array ID materi
	LastActivity  time.Time
}