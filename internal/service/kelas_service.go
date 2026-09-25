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
	return &KelasService{repo: repo, modulRepo: modulRepo}
}

func (s *KelasService) CreateKelas(guruID uint, nama string, mataPelajaran string) (*model.Kelas, error) {
	if nama == "" {
		return nil, fmt.Errorf("nama kelas wajib diisi")
	}

	var kode string
	for {
		kode = kodegenerator.GenerateKodeKelas()
		exists, err := s.repo.IsKodeExists(kode)
		if err != nil {
			return nil, fmt.Errorf("gagal mengecek kode kelas: %w", err)
		}
		if !exists {
			break
		}
	}

	kelas := &model.Kelas{
		GuruID:        guruID,
		NamaKelas:     nama,
		MataPelajaran: mataPelajaran,
		KodeKelas:     kode,
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

func (s *KelasService) UpdateKelas(id, guruID uint, nama string, mataPelajaran string) (*model.Kelas, error) {
	kelas, err := s.repo.FindByIDAndGuruID(id, guruID)
	if err != nil || kelas == nil {
		return nil, fmt.Errorf("kelas tidak ditemukan atau Anda tidak memiliki akses")
	}

	if nama != "" {
		kelas.NamaKelas = nama
	}
	kelas.MataPelajaran = mataPelajaran

	err = s.repo.Update(kelas)
	if err != nil {
		return nil, fmt.Errorf("gagal update kelas: %w", err)
	}

	return kelas, nil
}

// DeleteKelas - CASCADE: hapus kelas + siswa + jawaban + progress + relasi modul
func (s *KelasService) DeleteKelas(id, guruID uint) error {
	kelas, err := s.repo.FindByIDAndGuruID(id, guruID)
	if err != nil || kelas == nil {
		return fmt.Errorf("kelas tidak ditemukan atau Anda tidak memiliki akses")
	}

	err = s.repo.DeleteWithCascade(id)
	if err != nil {
		return fmt.Errorf("gagal menghapus kelas: %w", err)
	}

	return nil
}

func (s *KelasService) AssignModul(kelasID, modulID, guruID uint) error {
	kelas, err := s.repo.FindByIDAndGuruID(kelasID, guruID)
	if err != nil || kelas == nil {
		return fmt.Errorf("kelas tidak ditemukan atau Anda tidak memiliki akses ke kelas ini")
	}

	modul, err := s.modulRepo.FindByIDAndGuruID(modulID, guruID)
	if err != nil || modul == nil {
		return fmt.Errorf("modul tidak ditemukan atau Anda tidak memiliki akses ke modul ini")
	}

	return s.repo.AssignModul(kelas, modul)
}

func (s *KelasService) RemoveModul(kelasID, modulID, guruID uint) error {
	kelas, err := s.repo.FindByIDAndGuruID(kelasID, guruID)
	if err != nil || kelas == nil {
		return fmt.Errorf("kelas tidak ditemukan atau Anda tidak memiliki akses")
	}

	return s.repo.RemoveModul(kelas, modulID)
}

func (s *KelasService) JoinKelasWithKode(kodeKelas, namaSiswa string) (*model.Kelas, error) {
	kelas, err := s.repo.FindByKodeKelas(kodeKelas)
	if err != nil {
		return nil, fmt.Errorf("kode kelas tidak valid")
	}

	return kelas, nil
}

func (s *KelasService) GetKelasByGuruIDPaginated(guruID uint, limit, offset int) ([]model.Kelas, int64, error) {
	return s.repo.FindByGuruIDWithPagination(guruID, limit, offset)
}