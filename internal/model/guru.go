package model

import "time"

type Guru struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	Nama              string    `gorm:"type:varchar(100);not null" json:"nama"`
	Email             string    `gorm:"type:varchar(100);uniqueIndex;not null" json:"email"`
	Password          string    `gorm:"type:varchar(255);not null" json:"-"`
	EmailVerified     bool      `gorm:"not null;default:false" json:"email_verified"`
	VerificationToken string    `gorm:"type:varchar(255)" json:"-"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type RegisterGuruRequest struct {
	Nama            string `json:"nama" binding:"required"`
	Email           string `json:"email" binding:"required,email"`
	Password        string `json:"password" binding:"required,min=6"`
	ConfirmPassword string `json:"confirm_password" binding:"required,eqfield=Password"`
}

type LoginGuruRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type LoginGuruResponse struct {
	Token string `json:"token"`
	Guru  Guru   `json:"guru"`
}