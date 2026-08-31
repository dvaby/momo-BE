package service

import (
	"fmt"

	"momo-be/internal/model"
	"momo-be/internal/repository"
	"momo-be/pkg/jwtutil"
)

type SiswaService struct {
	repo      *repository.SiswaRepository
	kelasRepo *repository.KelasRepository
}

func NewSiswaService(repo *repository.SiswaRepository, kelasRepo *repository.KelasRepository) *SiswaService {
	return &SiswaService{repo: repo, kelasRepo: kelasRepo}
}

func (s *SiswaService) DaftarkanSiswa(kelasID uint, nama string) (*model.Siswa, error) {
	_, err := s.kelasRepo.FindByID(kelasID)
	if err != nil {
		return nil, fmt.Errorf("kelas dengan ID %d tidak ditemukan", kelasID)
	}

	siswaSudahAda, err := s.repo.FindByNamaAndKelasID(nama, kelasID)
	if err == nil && siswaSudahAda != nil {
		return nil, fmt.Errorf("siswa dengan nama '%s'sudah terdaftar di kelas ini", nama)
	}

	siswa := &model.Siswa{
		KelasID: kelasID,
		Nama:    nama,
	}

	err = s.repo.Create(siswa)
	if err != nil {
		return nil, fmt.Errorf("gagal mendaftarkan siswa: %w", err)
	}

	return siswa, nil
}

// JoinSiswa dipanggil saat siswa (lewat suara) sebut kode kelas + nama.
// Alurnya: cari Kelas berdasarkan kode -> cari Siswa dengan nama itu
// di dalam kelas tersebut -> kalau cocok, terbitkan token JWT sebagai
// bukti identitas siswa itu untuk request-request selanjutnya.
func (s *SiswaService) JoinSiswa(kodeKelas string, nama string) (*model.Siswa, string, error) {
	kelas, err := s.kelasRepo.FindByKode(kodeKelas)
	if err != nil {
		return nil, "", fmt.Errorf("kelas dengan kode '%s' tidak ditemukan", kodeKelas)
	}

	siswa, err := s.repo.FindByNamaAndKelasID(nama, kelas.ID)
	if err != nil || siswa == nil {
		return nil, "", fmt.Errorf("nama '%s' tidak terdaftar di kelas ini", nama)
	}

	token, err := jwtutil.GenerateToken(siswa.ID, kelas.ID)
	if err != nil {
		return nil, "", fmt.Errorf("gagal membuat token: %w", err)
	}

	return siswa, token, nil
}