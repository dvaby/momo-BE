package service

import (
	"fmt"

	"momo-be/internal/model"
	"momo-be/internal/repository"
	"momo-be/pkg/kodegenerator"
)

type KelasService struct {
	repo      *repository.KelasRepository
	modulRepo repository.ModulRepository
}

func NewKelasService(repo *repository.KelasRepository, modulRepo repository.ModulRepository) *KelasService {
	return &KelasService{
		repo:      repo,
		modulRepo: modulRepo,
	}
}

func (s *KelasService) CreateKelas(guruID uint, namaKelas string) (*model.Kelas, error) {
	kode, err := s.generateKodeUnik()
	if err != nil {
		return nil, err
	}

	kelas := &model.Kelas{
		GuruID:    guruID,
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

func (s *KelasService) GetKelasByID(id uint, guruID uint) (*model.Kelas, error) {
	return s.repo.FindByIDAndGuruID(id, guruID)
}

func (s *KelasService) AssignModul(kelasID uint, modulID uint, guruID uint) error {
	kelas, err := s.repo.FindByIDAndGuruID(kelasID, guruID)
	if err != nil {
		return fmt.Errorf("kelas tidak ditemukan atau bukan milik guru ini: %w", err)
	}

	modul, err := s.modulRepo.FindByIDAndGuruID(modulID, guruID)
	if err != nil {
		return fmt.Errorf("modul tidak ditemukan atau bukan milik guru ini: %w", err)
	}

	err = s.repo.AssignModul(kelas, modul)
	if err != nil {
		return fmt.Errorf("gagal mengaitkan modul ke kelas: %w", err)
	}

	return nil
}