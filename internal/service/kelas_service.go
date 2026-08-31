package service

import (
	"crypto/rand"
	"fmt"
	"math/big"

	"momo-be/internal/model"
	"momo-be/internal/repository"
)

type KelasService struct {
	repo      *repository.KelasRepository
	modulRepo repository.ModulRepository
}

func NewKelasService(repo *repository.KelasRepository, modulRepo repository.ModulRepository) *KelasService {
	return &KelasService{repo: repo, modulRepo: modulRepo}
}

func generateRandomCode(length int) string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[num.Int64()]
	}
	return string(b)
}

func (s *KelasService) CreateKelas(guruID uint, nama string) (*model.Kelas, error) {
	if nama == "" {
		return nil, fmt.Errorf("nama kelas wajib diisi")
	}

	var kode string
	for {
		kode = generateRandomCode(6)
		exists, err := s.repo.IsKodeExists(kode)
		if err != nil {
			return nil, fmt.Errorf("gagal mengecek kode kelas: %w", err)
		}
		if !exists {
			break
		}
	}

	kelas := &model.Kelas{
		GuruID:    guruID,
		NamaKelas: nama,
		KodeKelas: kode,
	}

	err := s.repo.Create(kelas)
	if err != nil {
		return nil, fmt.Errorf("gagal menyimpan kelas: %w", err)
	}

	return kelas, nil
}

func (s *KelasService) GetKelasByGuruID(guruID uint) ([]model.Kelas, error) {
	return s.repo.FindByGuruID(guruID)
}

func (s *KelasService) GetKelasByID(id, guruID uint) (*model.Kelas, error) {
	kelas, err := s.repo.FindByIDAndGuruID(id, guruID)
	if err != nil || kelas == nil {
		return nil, fmt.Errorf("kelas tidak ditemukan atau Anda tidak memiliki akses")
	}
	return kelas, nil
}

func (s *KelasService) AssignModul(kelasID, modulID, guruID uint) error {
	// 1. Validasi kepemilikan Kelas oleh Guru
	kelas, err := s.repo.FindByIDAndGuruID(kelasID, guruID)
	if err != nil || kelas == nil {
		return fmt.Errorf("kelas tidak ditemukan atau Anda tidak memiliki akses ke kelas ini")
	}

	// 2. Validasi kepemilikan Modul oleh Guru (Mencegah Guru A memakai Modul Guru B)
	modul, err := s.modulRepo.FindByIDAndGuruID(modulID, guruID)
	if err != nil || modul == nil {
		return fmt.Errorf("modul tidak ditemukan atau Anda tidak memiliki akses ke modul ini")
	}

	return s.repo.AssignModul(kelas, modul)
}
