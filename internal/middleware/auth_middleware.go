package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"momo-be/pkg/jwtutil"
)

// AuthMiddleware melindungi endpoint khusus Siswa.
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token tidak ditemukan. Silakan login kembali.", "code": "TOKEN_MISSING"})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Format header Authorization harus 'Bearer <token>'", "code": "TOKEN_INVALID_FORMAT"})
			c.Abort()
			return
		}

		tokenString := parts[1]

		claims, err := jwtutil.VerifyToken(tokenString)
		if err != nil {
			errMsg := "Token siswa tidak valid. Silakan login kembali."
			if strings.Contains(err.Error(), "expired") {
				errMsg = "Sesi Anda telah berakhir. Silakan login kembali."
			}
			c.JSON(http.StatusUnauthorized, gin.H{"error": errMsg, "code": "TOKEN_INVALID"})
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
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token tidak ditemukan. Silakan login kembali.", "code": "TOKEN_MISSING"})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Format header Authorization harus 'Bearer <token>'", "code": "TOKEN_INVALID_FORMAT"})
			c.Abort()
			return
		}

		tokenString := parts[1]

		claims, err := jwtutil.VerifyGuruToken(tokenString)
		if err != nil {
			errMsg := "Token guru tidak valid. Silakan login kembali."
			if strings.Contains(err.Error(), "expired") {
				errMsg = "Sesi Anda telah berakhir. Silakan login kembali."
			}
			c.JSON(http.StatusUnauthorized, gin.H{"error": errMsg, "code": "TOKEN_INVALID"})
			c.Abort()
			return
		}

		c.Set("guru_id", claims.GuruID)
		c.Set("role", claims.Role)

		c.Next()
	}
}
