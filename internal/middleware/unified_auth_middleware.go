package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"momo-be/pkg/jwtutil"
)

// UnifiedAuthMiddleware menerima token guru ATAU token siswa.
// Token bisa dikirim via:
//  1. Header: Authorization: Bearer <token>  (preferred, untuk fetch-based client)
//  2. Query param: ?token=<token>            (fallback, untuk EventSource native)
//
// Set context: guru_id (jika guru) ATAU siswa_id+kelas_id (jika siswa) + role.
func UnifiedAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := ""

		// Sumber 1: header Authorization
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "Format header Authorization harus 'Bearer <token>'",
					"code":  "TOKEN_INVALID_FORMAT",
				})
				c.Abort()
				return
			}
			tokenStr = parts[1]
		} else {
			// Sumber 2: query param ?token= (untuk EventSource native)
			tokenStr = c.Query("token")
		}

		if tokenStr == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Token tidak ditemukan. Silakan login terlebih dahulu.",
				"code":  "TOKEN_MISSING",
			})
			c.Abort()
			return
		}

		// Coba verify sebagai token guru dulu
		guruClaims, err := jwtutil.VerifyGuruToken(tokenStr)
		if err == nil && guruClaims.GuruID > 0 {
			c.Set("guru_id", guruClaims.GuruID)
			c.Set("role", "guru")
			c.Next()
			return
		}

		// Coba verify sebagai token siswa
		siswaClaims, err := jwtutil.VerifyToken(tokenStr)
		if err == nil && siswaClaims.SiswaID > 0 {
			c.Set("siswa_id", siswaClaims.SiswaID)
			c.Set("kelas_id", siswaClaims.KelasID)
			c.Set("role", "siswa")
			c.Next()
			return
		}

		// Dua-duanya gagal
		errMsg := "Token tidak valid. Silakan login kembali."
		if err != nil && strings.Contains(err.Error(), "expired") {
			errMsg = "Sesi Anda telah berakhir. Silakan login kembali."
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": errMsg, "code": "TOKEN_INVALID"})
		c.Abort()
	}
}