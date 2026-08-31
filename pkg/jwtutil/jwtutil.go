package jwtutil

import (
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// SiswaClaims adalah data yang dititipkan di token siswa.
type SiswaClaims struct {
	SiswaID uint `json:"siswa_id"`
	KelasID uint `json:"kelas_id"`
	jwt.RegisteredClaims
}

// GuruClaims adalah data yang dititipkan di token guru.
type GuruClaims struct {
	GuruID uint   `json:"guru_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// GenerateToken membuat token JWT baru untuk siswa.
func GenerateToken(siswaID uint, kelasID uint) (string, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return "", errors.New("JWT_SECRET tidak ditemukan di environment")
	}

	claims := SiswaClaims{
		SiswaID: siswaID,
		KelasID: kelasID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(12 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// VerifyToken memeriksa token siswa.
func VerifyToken(tokenString string) (*SiswaClaims, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return nil, errors.New("JWT_SECRET tidak ditemukan di environment")
	}

	claims := &SiswaClaims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("token tidak valid")
	}

	return claims, nil
}

// GenerateGuruToken membuat token JWT khusus untuk guru (berlaku 24 jam).
func GenerateGuruToken(guruID uint) (string, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return "", errors.New("JWT_SECRET tidak ditemukan di environment")
	}

	claims := GuruClaims{
		GuruID: guruID,
		Role:   "guru",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// VerifyGuruToken memeriksa token guru dan memastikan role-nya "guru".
func VerifyGuruToken(tokenString string) (*GuruClaims, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return nil, errors.New("JWT_SECRET tidak ditemukan di environment")
	}

	claims := &GuruClaims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("token tidak valid")
	}

	if claims.Role != "guru" {
		return nil, errors.New("token bukan milik role guru")
	}

	return claims, nil
}