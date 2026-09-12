package service

import (
	"fmt"
	"strings"

	"momo-be/internal/model"
	"momo-be/internal/repository"
	"momo-be/pkg/aiclient"
	"momo-be/pkg/pdfworker"
)

type MateriService struct {
	repo      *repository.MateriRepository
	modulRepo repository.ModulRepository
	aiClient  *aiclient.Client
}

func NewMateriService(repo *repository.MateriRepository, modulRepo repository.ModulRepository, aiClient *aiclient.Client) *MateriService {
	return &MateriService{repo: repo, modulRepo: modulRepo, aiClient: aiClient}
}

func (s *MateriService) ValidateModulOwnership(modulID uint, guruID uint) error {
	_, err := s.modulRepo.FindByIDAndGuruID(modulID, guruID)
	if err != nil {
		return fmt.Errorf("akses ditolak: modul tidak ditemukan atau bukan milik Anda")
	}
	return nil
}

func (s *MateriService) ProcessAndSaveMateri(modulID uint, pdfFilePath string) ([]model.Materi, error) {
	teksMentah, err := pdfworker.ExtractText(pdfFilePath)
	if err != nil {
		return nil, fmt.Errorf("gagal ekstrak PDF: %w", err)
	}

	aiResponse, err := s.aiClient.ProcessText("materi", teksMentah)
	if err != nil {
		return nil, fmt.Errorf("gagal memproses lewat AI Service: %w", err)
	}

	if !aiResponse.Success {
		errMsg := "AI Service gagal memproses teks"
		if aiResponse.Message != "" {
			errMsg = aiResponse.Message
		}
		return nil, fmt.Errorf(errMsg)
	}

	var materiList []model.Materi
	for _, item := range aiResponse.Data.Materi {
		materiList = append(materiList, model.Materi{
			ModulID: modulID,
			Urutan:  item.Urutan,
			Judul:   item.Judul,
			Konten:  item.Konten,
		})
	}

	if len(materiList) == 0 {
		return nil, fmt.Errorf("AI Service tidak menemukan konten materi yang valid dari PDF ini — pastikan PDF berisi materi pembelajaran, bukan soal")
	}

	err = s.repo.CreateBatch(materiList)
	if err != nil {
		return nil, fmt.Errorf("gagal menyimpan materi ke database: %w", err)
	}

	return materiList, nil
}

// GetByModulID mengambil daftar materi milik modul (dengan cek kepemilikan guru)
func (s *MateriService) GetByModulID(modulID uint, guruID uint) ([]model.Materi, error) {
	if err := s.ValidateModulOwnership(modulID, guruID); err != nil {
		return nil, err
	}
	return s.repo.FindByModulID(modulID)
}

// CreateManual membuat materi tulisan tangan (tanpa PDF/AI)
func (s *MateriService) CreateManual(modulID uint, guruID uint, urutan int, judul string, konten string) (*model.Materi, error) {
	if err := s.ValidateModulOwnership(modulID, guruID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(judul) == "" || strings.TrimSpace(konten) == "" {
		return nil, fmt.Errorf("judul dan konten materi wajib diisi")
	}
	if urutan <= 0 {
		existing, err := s.repo.FindByModulID(modulID)
		if err != nil {
			return nil, err
		}
		urutan = len(existing) + 1
	}
	materi := &model.Materi{
		ModulID: modulID,
		Urutan:  urutan,
		Judul:   judul,
		Konten:  konten,
	}
	if err := s.repo.Create(materi); err != nil {
		return nil, fmt.Errorf("gagal menyimpan materi: %w", err)
	}
	return materi, nil
}

// validateMateriOwnership memastikan materi ada dan modul induknya milik guru ini
func (s *MateriService) validateMateriOwnership(materiID uint, guruID uint) (*model.Materi, error) {
	materi, err := s.repo.FindByID(materiID)
	if err != nil {
		return nil, fmt.Errorf("materi tidak ditemukan")
	}
	if _, err := s.modulRepo.FindByIDAndGuruID(materi.ModulID, guruID); err != nil {
		return nil, fmt.Errorf("akses ditolak: materi ini bukan milik Anda")
	}
	return materi, nil
}

// Update mengubah judul/konten/urutan materi
func (s *MateriService) Update(materiID uint, guruID uint, judul string, konten string, urutan int) (*model.Materi, error) {
	materi, err := s.validateMateriOwnership(materiID, guruID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(judul) == "" || strings.TrimSpace(konten) == "" {
		return nil, fmt.Errorf("judul dan konten materi wajib diisi")
	}
	materi.Judul = judul
	materi.Konten = konten
	if urutan > 0 {
		materi.Urutan = urutan
	}
	if err := s.repo.Update(materi); err != nil {
		return nil, fmt.Errorf("gagal memperbarui materi: %w", err)
	}
	return materi, nil
}

// Delete menghapus satu materi
func (s *MateriService) Delete(materiID uint, guruID uint) error {
	materi, err := s.validateMateriOwnership(materiID, guruID)
	if err != nil {
		return err
	}
	return s.repo.Delete(materi.ID)
}
