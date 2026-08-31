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
	modulRepo repository.ModulRepository
}

func NewNilaiService(repo *repository.NilaiRepository, kelasRepo *repository.KelasRepository, modulRepo repository.ModulRepository) *NilaiService {
	return &NilaiService{
		repo:      repo,
		kelasRepo: kelasRepo,
		modulRepo: modulRepo,
	}
}

func (s *NilaiService) GetRekapNilai(kelasID uint, modulID uint, jenis string, guruID uint) ([]model.NilaiSiswa, error) {
	if modulID == 0 {
		return nil, errors.New("modul_id wajib diisi dan harus valid")
	}

	// 1. Validasi kepemilikan Kelas
	_, err := s.kelasRepo.FindByIDAndGuruID(kelasID, guruID)
	if err != nil {
		return nil, fmt.Errorf("akses ditolak: kelas tidak ditemukan atau bukan milik Anda")
	}

	// 2. Validasi kepemilikan Modul
	modul, err := s.modulRepo.FindByIDAndGuruID(modulID, guruID)
	if err != nil || modul == nil {
		return nil, fmt.Errorf("akses ditolak: modul tidak ditemukan atau bukan milik Anda")
	}

	return s.repo.GetRekapNilai(kelasID, modulID, jenis)
}
