package service

import (
	"errors"
	"momo-be/internal/model"
	"momo-be/internal/repository"
)

type NilaiService struct {
	repo *repository.NilaiRepository
}

func NewNilaiService(repo *repository.NilaiRepository) *NilaiService {
	return &NilaiService{repo: repo}
}

func (s *NilaiService) GetRekapNilai(kelasID uint, modulID uint, jenis string) ([]model.NilaiSiswa, error) {
	// Validasi sederhana: modulID wajib ada (> 0)
	if modulID == 0 {
		return nil, errors.New("modul_id wajib diisi dan harus valid")
	}

	return s.repo.GetRekapNilai(kelasID, modulID, jenis)
}