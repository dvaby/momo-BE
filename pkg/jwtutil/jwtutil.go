package jwtutil

import (
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// SiswaClaims adalah data yang kita "titipkan" di dalam token.
// Selain field custom kita, kita embed jwt.RegisteredClaims supaya
// dapat field standar seperti ExpiresAt (exp) secara otomatis.
type SiswaClaims struct {
	SiswaID uint `json:"siswa_id"`
	KelasID uint `json:"kelas_id"`
	jwt.RegisteredClaims
}

// GenerateToken membuat token JWT baru untuk siswa yang baru saja
// berhasil "join" (kode kelas + nama-nya cocok).
// Token ini akan dipakai FE sebagai bukti identitas di request berikutnya.
func GenerateToken(siswaID uint, kelasID uint) (string, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return "", errors.New("JWT_SECRET tidak ditemukan di environment")
	}

	claims := SiswaClaims{
		SiswaID: siswaID,
		KelasID: kelasID,
		RegisteredClaims: jwt.RegisteredClaims{
			// Token berlaku 12 jam sejak diterbitkan.
			// Cukup panjang untuk satu sesi belajar, tapi tidak selamanya.
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(12 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// VerifyToken memeriksa apakah token asli (ditandatangani dengan secret
// yang benar) dan belum kedaluwarsa. Kalau valid, kembalikan isi claims-nya
// (siswa_id, kelas_id) supaya handler tahu ini request dari siswa yang mana.
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