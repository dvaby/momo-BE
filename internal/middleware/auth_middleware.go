package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"momo-be/pkg/jwtutil"
)

// AuthMiddleware memeriksa header "Authorization: Bearer <token>" pada
// setiap request yang masuk ke endpoint yang dilindungi. Kalau token
// tidak ada, salah format, atau tidak valid (dipalsukan/kedaluwarsa),
// request langsung ditolak sebelum sampai ke handler.
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "header Authorization tidak ditemukan"})
			c.Abort()
			return
		}

		// Header harus berformat "Bearer <token>", jadi kita pecah
		// jadi 2 bagian dan pastikan bagian pertamanya persis "Bearer".
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "format header Authorization harus 'Bearer <token>'"})
			c.Abort()
			return
		}

		tokenString := parts[1]

		claims, err := jwtutil.VerifyToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token tidak valid atau sudah kedaluwarsa"})
			c.Abort()
			return
		}

		// Simpan identitas siswa ke context, supaya handler di belakang
		// middleware ini bisa langsung ambil siswa_id & kelas_id yang
		// TERPERCAYA (dari token yang sudah diverifikasi), bukan dari
		// body/param yang bisa dipalsukan siswa.
		c.Set("siswa_id", claims.SiswaID)
		c.Set("kelas_id", claims.KelasID)

		c.Next()
	}
}