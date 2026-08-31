package service

import (
	"errors"
	"fmt"

	"momo-be/internal/model"
	"momo-be/internal/repository"
)

type NilaiService struct {
	repo      *repository.NilaiRepository
	kelasRepo *repository.KelasRepository
}

func NewNilaiService(repo *repository.NilaiRepository, kelasRepo *repository.KelasRepository) *NilaiService {
	return &NilaiService{
		repo:      repo,
		kelasRepo: kelasRepo,
	}
}

func (s *NilaiService) GetRekapNilai(kelasID uint, modulID uint, jenis string, guruID uint) ([]model.NilaiSiswa, error) {
	if modulID == 0 {
		return nil, errors.New("modul_id wajib diisi dan harus valid")
	}

	// Memakai FindByIDAndGuruID
	_, err := s.kelasRepo.FindByIDAndGuruID(kelasID, guruID)
	if err != nil {
		return nil, fmt.Errorf("akses ditolak: kelas tidak ditemukan atau bukan milik Anda")
	}

	return s.repo.GetRekapNilai(kelasID, modulID, jenis)
}