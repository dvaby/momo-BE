package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"momo-be/internal/errors"
	"momo-be/internal/repository"
	"momo-be/pkg/jwtutil"
)

// NewAuthMiddleware melindungi endpoint khusus Siswa.
// Identitas siswa bisa lewat DUA cara (19 Sept: token TIDAK wajib lagi):
//  1. Header X-Session-ID: <session_id>  ← alur suara tanpa token (UTAMA)
//  2. Header Authorization: Bearer <token siswa>  ← legacy/fallback
func NewAuthMiddleware(siswaRepo *repository.SiswaRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Cara 1 (UTAMA): session ID
		sessionID := c.GetHeader("X-Session-ID")
		if sessionID == "" {
			sessionID = c.Query("session_id")
		}
		if sessionID != "" && siswaRepo != nil {
			siswa, err := siswaRepo.FindBySessionID(sessionID)
			if err == nil && siswa != nil {
				c.Set("siswa_id", siswa.ID)
				c.Set("kelas_id", siswa.KelasID)
				c.Next()
				return
			}
		}

		// Cara 2 (legacy): token JWT siswa
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				c.JSON(http.StatusUnauthorized, errors.NewClientError(errors.CodeInvalidFormat, "Format header Authorization harus 'Bearer <token>'"))
				c.Abort()
				return
			}
			claims, err := jwtutil.VerifyToken(parts[1])
			if err != nil {
				errMsg := "Token siswa tidak valid. Silakan login kembali."
				if strings.Contains(err.Error(), "expired") {
					errMsg = "Sesi Anda telah berakhir. Silakan login kembali."
				}
				c.JSON(http.StatusUnauthorized, errors.NewClientError(errors.CodeUnauthorized, errMsg))
				c.Abort()
				return
			}
			if claims.SiswaID == 0 {
				c.JSON(http.StatusForbidden, errors.NewClientError(errors.CodeForbidden, "Endpoint ini khusus siswa. Token Anda bukan token siswa."))
				c.Abort()
				return
			}
			c.Set("siswa_id", claims.SiswaID)
			c.Set("kelas_id", claims.KelasID)
			c.Next()
			return
		}

		c.JSON(http.StatusUnauthorized, errors.NewClientError(errors.CodeUnauthorized, "Identitas siswa tidak ditemukan. Mulai percakapan dengan Momo terlebih dahulu."))
		c.Abort()
	}
}

// GuruAuthMiddleware melindungi endpoint khusus Guru.
func GuruAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, errors.NewClientError(errors.CodeUnauthorized, "Token tidak ditemukan. Silakan login terlebih dahulu."))
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, errors.NewClientError(errors.CodeInvalidFormat, "Format header Authorization harus 'Bearer <token>'"))
			c.Abort()
			return
		}

		claims, err := jwtutil.VerifyGuruToken(parts[1])
		if err != nil {
			errMsg := "Token guru tidak valid. Silakan login kembali."
			if strings.Contains(err.Error(), "expired") {
				errMsg = "Sesi Anda telah berakhir. Silakan login kembali."
			}
			c.JSON(http.StatusUnauthorized, errors.NewClientError(errors.CodeUnauthorized, errMsg))
			c.Abort()
			return
		}

		if claims.GuruID == 0 {
			c.JSON(http.StatusForbidden, errors.NewClientError(errors.CodeForbidden, "Endpoint ini khusus guru. Token Anda bukan token guru."))
			c.Abort()
			return
		}

		c.Set("guru_id", claims.GuruID)
		c.Set("role", claims.Role)
		c.Next()
	}
}