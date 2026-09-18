package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"momo-be/internal/errors"
	"momo-be/pkg/jwtutil"
)

// AuthMiddleware melindungi endpoint khusus Siswa.
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, errors.NewClientError(
				errors.CodeUnauthorized, "Token tidak ditemukan. Silakan login terlebih dahulu."))
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, errors.NewClientError(
				errors.CodeInvalidFormat, "Format header Authorization harus 'Bearer <token>'"))
			c.Abort()
			return
		}

		claims, err := jwtutil.VerifyToken(parts[1])
		if err != nil {
			errMsg := "Token siswa tidak valid. Silakan login kembali."
			if strings.Contains(err.Error(), "expired") {
				errMsg = "Sesi Anda telah berakhir. Silakan login kembali."
			}
			c.JSON(http.StatusUnauthorized, errors.NewClientError(
				errors.CodeUnauthorized, errMsg))
			c.Abort()
			return
		}

		if claims.SiswaID == 0 {
			c.JSON(http.StatusForbidden, errors.NewClientError(
				errors.CodeForbidden, "Endpoint ini khusus siswa. Token Anda bukan token siswa."))
			c.Abort()
			return
		}

		c.Set("siswa_id", claims.SiswaID)
		c.Set("kelas_id", claims.KelasID)
		c.Next()
	}
}

// GuruAuthMiddleware melindungi endpoint khusus Guru.
func GuruAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, errors.NewClientError(
				errors.CodeUnauthorized, "Token tidak ditemukan. Silakan login terlebih dahulu."))
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, errors.NewClientError(
				errors.CodeInvalidFormat, "Format header Authorization harus 'Bearer <token>'"))
			c.Abort()
			return
		}

		claims, err := jwtutil.VerifyGuruToken(parts[1])
		if err != nil {
			errMsg := "Token guru tidak valid. Silakan login kembali."
			if strings.Contains(err.Error(), "expired") {
				errMsg = "Sesi Anda telah berakhir. Silakan login kembali."
			}
			c.JSON(http.StatusUnauthorized, errors.NewClientError(
				errors.CodeUnauthorized, errMsg))
			c.Abort()
			return
		}

		if claims.GuruID == 0 {
			c.JSON(http.StatusForbidden, errors.NewClientError(
				errors.CodeForbidden, "Endpoint ini khusus guru. Token Anda bukan token guru."))
			c.Abort()
			return
		}

		c.Set("guru_id", claims.GuruID)
		c.Set("role", claims.Role)
		c.Next()
	}
}