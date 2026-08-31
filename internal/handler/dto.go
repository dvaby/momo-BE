package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"momo-be/internal/model"
)

// DTO Response Soal Gabungan
type SoalResponse struct {
	ID         uint      `json:"id"`
	ModulID    uint      `json:"modul_id"`
	Jenis      string    `json:"jenis"`
	Pertanyaan string    `json:"pertanyaan"`
	PilihanA   string    `json:"pilihan_a"`
	PilihanB   string    `json:"pilihan_b"`
	PilihanC   string    `json:"pilihan_c"`
	PilihanD   string    `json:"pilihan_d"`
	CreatedAt  time.Time `json:"created_at,omitempty"`
}

func ToSoalResponse(soal model.Soal) SoalResponse {
	return SoalResponse{
		ID:         soal.ID,
		ModulID:    soal.ModulID,
		Jenis:      string(soal.Jenis),
		Pertanyaan: soal.Pertanyaan,
		PilihanA:   soal.PilihanA,
		PilihanB:   soal.PilihanB,
		PilihanC:   soal.PilihanC,
		PilihanD:   soal.PilihanD,
		CreatedAt:  soal.CreatedAt,
	}
}

type ModulDetailResponse struct {
	ID        uint           `json:"id"`
	Judul     string         `json:"judul"`
	Deskripsi string         `json:"deskripsi"`
	Materi    []model.Materi `json:"materi,omitempty"`
	Soal      []SoalResponse `json:"soal,omitempty"`
}

func ToModulDetailResponse(modul model.Modul) ModulDetailResponse {
	soalResponse := make([]SoalResponse, 0, len(modul.Soal))
	for _, soal := range modul.Soal {
		soalResponse = append(soalResponse, ToSoalResponse(soal))
	}

	return ModulDetailResponse{
		ID:        modul.ID,
		Judul:     modul.Nama,
		Deskripsi: modul.Deskripsi,
		Materi:    modul.Materi,
		Soal:      soalResponse,
	}
}

// Helper untuk mengambil dan melakukan casting tipe data dari Gin Context dengan aman
func getUintFromContext(c *gin.Context, key string) (uint, bool) {
	val, exists := c.Get(key)
	if !exists {
		return 0, false
	}
	switch v := val.(type) {
	case float64:
		return uint(v), true
	case uint:
		return v, true
	case int:
		return uint(v), true
	default:
		return 0, false
	}
}