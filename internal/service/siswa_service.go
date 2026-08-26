package service

import (
	"fmt"

	"momo-be/internal/model"
	"momo-be/internal/repository"
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
		return nil, fmt.Errorf("siswa dengan nama '%s' sudah terdaftar di kelas ini", nama)
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