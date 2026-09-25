package repository

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"momo-be/internal/model"
)

type SessionStateRepository interface {
	Get(sessionID string) (string, error)
	Save(sessionID string, jsonData string) error
	Delete(sessionID string) error
}

type sessionStateRepo struct{ db *gorm.DB }

func NewSessionStateRepository(db *gorm.DB) SessionStateRepository {
	return &sessionStateRepo{db: db}
}

func (r *sessionStateRepo) Get(sessionID string) (string, error) {
	var rec model.SessionRecord
	if err := r.db.First(&rec, "session_id = ?", sessionID).Error; err != nil {
		return "", err
	}
	return rec.Data, nil
}

func (r *sessionStateRepo) Save(sessionID string, jsonData string) error {
	rec := model.SessionRecord{SessionID: sessionID, Data: jsonData, UpdatedAt: time.Now()}
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "session_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"data", "updated_at"}),
	}).Create(&rec).Error
}

func (r *sessionStateRepo) Delete(sessionID string) error {
	return r.db.Where("session_id = ?", sessionID).Delete(&model.SessionRecord{}).Error
}