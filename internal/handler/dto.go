package handler

import (
	"time"

	"momo-be/internal/model"
)

type SoalSiswaResponse struct {
	ID         uint      `json:"id"`
	ModulID    uint      `json:"modul_id"`
	Jenis      string    `json:"jenis"`
	Pertanyaan string    `json:"pertanyaan"`
	PilihanA   string    `json:"pilihan_a"`
	PilihanB   string    `json:"pilihan_b"`
	PilihanC   string    `json:"pilihan_c"`
	PilihanD   string    `json:"pilihan_d"`
	CreatedAt  time.Time `json:"created_at"`
}

func ToSoalSiswaResponse(soal model.Soal) SoalSiswaResponse {
	return SoalSiswaResponse{
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

type SoalTanpaKunciResponse struct {
	ID         uint   `json:"id"`
	ModulID    uint   `json:"modul_id"`
	Jenis      string `json:"jenis"`
	Pertanyaan string `json:"pertanyaan"`
	PilihanA   string `json:"pilihan_a"`
	PilihanB   string `json:"pilihan_b"`
	PilihanC   string `json:"pilihan_c"`
	PilihanD   string `json:"pilihan_d"`
}

type ModulDetailResponse struct {
	ID        uint                     `json:"id"`
	Judul     string                   `json:"judul"`
	Deskripsi string                   `json:"deskripsi"`
	Materi    []model.Materi           `json:"materi,omitempty"`
	Soal      []SoalTanpaKunciResponse `json:"soal,omitempty"`
}

func ToModulDetailResponse(modul model.Modul) ModulDetailResponse {
	soalResponse := make([]SoalTanpaKunciResponse, 0, len(modul.Soal))
	for _, soal := range modul.Soal {
		soalResponse = append(soalResponse, SoalTanpaKunciResponse{
			ID:         soal.ID,
			ModulID:    soal.ModulID,
			Jenis:      string(soal.Jenis),
			Pertanyaan: soal.Pertanyaan,
			PilihanA:   soal.PilihanA,
			PilihanB:   soal.PilihanB,
			PilihanC:   soal.PilihanC,
			PilihanD:   soal.PilihanD,
		})
	}

	return ModulDetailResponse{
		ID:        modul.ID,
		Judul:     modul.Nama,
		Deskripsi: modul.Deskripsi,
		Materi:    modul.Materi,
		Soal:      soalResponse,
	}
}