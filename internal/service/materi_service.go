package service

import (
	"fmt"

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