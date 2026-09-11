package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DBHost              string
	DBPort              string
	DBUser              string
	DBPassword          string
	DBName              string
	DBSSLMode           string
	ServerPort          string
	AIServiceURL        string
	CORSAllowedOrigins  string
	BrevoAPIKey         string
	BrevoSenderEmail    string
	BrevoSenderName     string
	AppBaseURL          string
	FEVerifyRedirectURL string
	JWTSecret           string
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

	feVerifyURL := os.Getenv("FE_VERIFY_REDIRECT_URL")
	if feVerifyURL == "" {
		feVerifyURL = "http://localhost:3000/email-verified"
	}

	return &Config{
		DBHost:              os.Getenv("DB_HOST"),
		DBPort:              os.Getenv("DB_PORT"),
		DBUser:              os.Getenv("DB_USER"),
		DBPassword:          os.Getenv("DB_PASSWORD"),
		DBName:              os.Getenv("DB_NAME"),
		DBSSLMode:           sslMode,
		ServerPort:          os.Getenv("SERVER_PORT"),
		AIServiceURL:        os.Getenv("AI_SERVICE_URL"),
		CORSAllowedOrigins:  corsOrigins,
		BrevoAPIKey:         os.Getenv("BREVO_API_KEY"),
		BrevoSenderEmail:    os.Getenv("BREVO_SENDER_EMAIL"),
		BrevoSenderName:     os.Getenv("BREVO_SENDER_NAME"),
		AppBaseURL:          appBaseURL,
		FEVerifyRedirectURL: feVerifyURL,
		JWTSecret:           os.Getenv("JWT_SECRET"),
	}
}