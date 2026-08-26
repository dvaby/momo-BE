package service

import (
	"fmt"

	"momo-be/internal/model"
	"momo-be/internal/repository"
	"momo-be/pkg/kodegenerator"
)

type KelasService struct {
	repo *repository.KelasRepository
}

func NewKelasService(repo *repository.KelasRepository) *KelasService {
	return &KelasService{repo: repo}
}

func (s *KelasService) CreateKelas(namaKelas string) (*model.Kelas, error) {
	kode, err := s.generateKodeUnik()
	if err != nil {
		return nil, err
	}

	kelas := &model.Kelas{
		NamaKelas: namaKelas,
		KodeKelas: kode,
	}

	err = s.repo.Create(kelas)
	if err != nil {
		return nil, fmt.Errorf("gagal menyimpan kelas: %w", err)
	}

	return kelas, nil
}

func (s *KelasService) generateKodeUnik() (string, error) {
	maxPercobaan := 10

	for i := 0; i < maxPercobaan; i++ {
		kode := kodegenerator.GenerateKodeKelas()

		sudahAda, err := s.repo.IsKodeExists(kode)
		if err != nil {
			return "", fmt.Errorf("gagal cek ketersediaan kode: %w", err)
		}

		if !sudahAda {
			return kode, nil
		}
	}

	return "", fmt.Errorf("gagal menemukan kode unik setelah %d percobaan", maxPercobaan)
}

func (s *KelasService) GetKelasByID(id uint) (*model.Kelas, error) {
	return s.repo.FindByID(id)
}