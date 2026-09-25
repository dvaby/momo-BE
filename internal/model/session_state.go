package model

import "time"


type SessionRecord struct {
	SessionID string    `gorm:"primaryKey;size:100"`
	Data      string    `gorm:"type:text"`
	UpdatedAt time.Time
}