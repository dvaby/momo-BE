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
			c.JSON(http.StatusUnauthorized, gin.H{"error": "header Authorization tidak ditemukan"})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "format header Authorization harus 'Bearer <token>'"})
			c.Abort()
			return
		}

		tokenString := parts[1]

		claims, err := jwtutil.VerifyToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token siswa tidak valid atau sudah kedaluwarsa"})
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
			c.JSON(http.StatusUnauthorized, gin.H{"error": "header Authorization tidak ditemukan"})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "format header Authorization harus 'Bearer <token>'"})
			c.Abort()
			return
		}

		tokenString := parts[1]

		claims, err := jwtutil.VerifyGuruToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token guru tidak valid atau sudah kedaluwarsa"})
			c.Abort()
			return
		}

		c.Set("guru_id", claims.GuruID)
		c.Set("role", claims.Role)

		c.Next()
	}
}