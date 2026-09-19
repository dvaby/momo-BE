package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"momo-be/pkg/jwtutil"
)

// UnifiedAuthMiddleware menerima token guru ATAU token siswa.
// Sejak 19 Sept 2026: token OPSIONAL (mode public untuk stream/demo).
//   - Token guru valid   → role "guru"
//   - Token siswa valid  → role "siswa" + siswa_id + kelas_id
//   - Token TIDAK ada    → role "public" (stream terbuka, tidak 401)
//   - Token ada tapi invalid/expired → 401 (tetap strict supaya bug FE ketahuan)
func UnifiedAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := ""

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
			tokenStr = c.Query("token")
		}

		// Tanpa token → mode public (stream terbuka untuk demo/listening)
		if tokenStr == "" {
			c.Set("role", "public")
			c.Next()
			return
		}

		// Coba verify sebagai token guru
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

		// Token ada tapi tidak valid → tetap tolak
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Token tidak valid. Silakan login kembali.",
			"code":  "TOKEN_INVALID",
		})
		c.Abort()
	}
}