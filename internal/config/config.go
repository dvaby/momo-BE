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
	AIInternalToken   string
	AICallbackBaseURL string
	 AIServiceAuthToken  string
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

	// BARU: default kosong dulu, wajib diisi sebelum dipakai (nanti di Fase 3 saat wiring)
	aiInternalToken := os.Getenv("AI_INTERNAL_TOKEN")
	aiCallbackBaseURL := os.Getenv("AI_CALLBACK_BASE_URL")
	if aiCallbackBaseURL == "" {
		// Default ke AppBaseURL kalau kosong (akan jadi https://momo-be-production.up.railway.app)
		aiCallbackBaseURL = appBaseURL
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
		AIInternalToken:     aiInternalToken,
		AICallbackBaseURL:   aiCallbackBaseURL,
		AIServiceAuthToken: os.Getenv("AI_SERVICE_AUTH_TOKEN"),
	}
}

// CallbackURLWithSecret mengembalikan URL callback lengkap dengan query param secret.
// AI Service akan POST ke URL ini persis seperti yang kita kirim di request,
// sehingga secret otomatis terbawa tanpa perlu header tambahan.
func (c *Config) CallbackURLWithSecret() string {
	if c.AICallbackBaseURL == "" {
		return ""
	}
	// Gunakan AIInternalToken sebagai secret di query param
	return c.AICallbackBaseURL + "/api/v1/internal/ai-callback?token=" + c.AIInternalToken
}

// ToolsExecURLWithSecret generates the full tools execute URL with token
func (c *Config) ToolsExecURLWithSecret() string {
	return c.AICallbackBaseURL + "/api/v1/internal/tools/execute?token=" + c.AIInternalToken
}
