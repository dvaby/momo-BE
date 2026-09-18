package service

import (
	"fmt"

	"momo-be/internal/model"
)

// validateSiswaKelas memastikan token siswa cocok dengan data siswa di database
func (s *SiswaService) validateSiswaKelas(siswaID, kelasID uint) error {
	siswa, err := s.repo.FindByID(siswaID)
	if err != nil || siswa == nil {
		return fmt.Errorf("data siswa tidak ditemukan")
	}
	if siswa.KelasID != kelasID {
		return fmt.Errorf("akses ditolak: token kelas tidak cocok dengan data siswa")
	}
	return nil
}

// GetMateriBelajar mengambil materi untuk siswa belajar.
// TIDAK BUTUH AUTH GURU. Validasi hanya: anggota kelas + modul ditugaskan ke kelas.
func (s *SiswaService) GetMateriBelajar(siswaID, kelasID, modulID uint) ([]model.Materi, error) {
	if err := s.validateSiswaKelas(siswaID, kelasID); err != nil {
		return nil, err
	}

	allowed, err := s.kelasRepo.IsModulInKelas(kelasID, modulID)
	if err != nil {
		return nil, fmt.Errorf("gagal memverifikasi akses modul: %w", err)
	}
	if !allowed {
		return nil, fmt.Errorf("modul ini tidak ditugaskan untuk kelas Anda")
	}

	return s.materiRepo.FindByModulID(modulID)
}

// GetModulDetailForSiswa mengambil detail modul untuk halaman belajar siswa.
// TIDAK BUTUH AUTH GURU.
func (s *SiswaService) GetModulDetailForSiswa(siswaID, kelasID, modulID uint) (map[string]interface{}, error) {
	if err := s.validateSiswaKelas(siswaID, kelasID); err != nil {
		return nil, err
	}

	kelas, err := s.kelasRepo.FindByID(kelasID)
	if err != nil {
		return nil, fmt.Errorf("kelas tidak ditemukan")
	}

	var modulNama, modulDeskripsi string
	found := false
	for _, m := range kelas.Modul {
		if m.ID == modulID {
			modulNama = m.Nama
			modulDeskripsi = m.Deskripsi
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("modul ini tidak ditugaskan untuk kelas Anda")
	}

	materiList, _ := s.materiRepo.FindByModulID(modulID)

	jenisSoalTersedia := make([]string, 0)
	totalSoal := 0
	for _, jenis := range []model.JenisSoal{model.JenisSoalHarian, model.JenisSoalUTS, model.JenisSoalUAS} {
		soalList, err := s.soalRepo.FindByModulAndJenis(modulID, jenis)
		if err == nil && len(soalList) > 0 {
			jenisSoalTersedia = append(jenisSoalTersedia, string(jenis))
			totalSoal += len(soalList)
		}
	}

	return map[string]interface{}{
		"id":                  modulID,
		"nama":                modulNama,
		"deskripsi":           modulDeskripsi,
		"jumlah_materi":       len(materiList),
		"jumlah_soal":         totalSoal,
		"jenis_soal_tersedia": jenisSoalTersedia,
	}, nil
}