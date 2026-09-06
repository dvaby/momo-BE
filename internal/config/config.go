package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DBHost             string
	DBPort             string
	DBUser             string
	DBPassword         string
	DBName             string
	DBSSLMode          string
	ServerPort         string
	AIServiceURL       string
	CORSAllowedOrigins string
	SMTPHost           string
	SMTPPort           string
	SMTPUsername       string
	SMTPPassword       string
	SMTPFromName       string
	AppBaseURL         string
}

func LoadConfig() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: .env file tidak ditemukan, menggunakan environment variable sistem")
	}

	corsOrigins := os.Getenv("CORS_ALLOWED_ORIGINS")
	if corsOrigins == "" {
		corsOrigins = "http://localhost:3000,http://localhost:5173"
	}

	sslMode := os.Getenv("DB_SSLMODE")
	if sslMode == "" {
		sslMode = "disable"
	}

	appBaseURL := os.Getenv("APP_BASE_URL")
	if appBaseURL == "" {
		appBaseURL = "http://localhost:8080"
	}

		smtpHost := os.Getenv("SMTP_HOST")
	if smtpHost == "" {
		smtpHost = "smtp.gmail.com"
	}
	smtpPort := os.Getenv("SMTP_PORT")
	if smtpPort == "" {
		smtpPort = "587"
	}

	return &Config{
		DBHost:             os.Getenv("DB_HOST"),
		DBPort:             os.Getenv("DB_PORT"),
		DBUser:             os.Getenv("DB_USER"),
		DBPassword:         os.Getenv("DB_PASSWORD"),
		DBName:             os.Getenv("DB_NAME"),
		DBSSLMode:          sslMode,
		ServerPort:         os.Getenv("SERVER_PORT"),
		AIServiceURL:       os.Getenv("AI_SERVICE_URL"),
		CORSAllowedOrigins: corsOrigins,
		AppBaseURL:         appBaseURL,
		SMTPHost:     		smtpHost,
		SMTPPort:     		smtpPort,
		SMTPUsername: 		os.Getenv("SMTP_USERNAME"),
		SMTPPassword: 		os.Getenv("SMTP_PASSWORD"),
		SMTPFromName: 		os.Getenv("SMTP_FROM_NAME"),
	}
}