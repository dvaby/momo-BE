package service

import (
	"fmt"

	"momo-be/internal/model"
	"momo-be/internal/repository"
	"momo-be/pkg/aiclient"
	"momo-be/pkg/pdfworker"
)

type MateriService struct {
	repo     *repository.MateriRepository
	aiClient *aiclient.Client
}

func NewMateriService(repo *repository.MateriRepository, aiClient *aiclient.Client) *MateriService {
	return &MateriService{repo: repo, aiClient: aiClient}
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
		return nil, fmt.Errorf("AI Service gagal memproses teks")
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

	err = s.repo.CreateBatch(materiList)
	if err != nil {
		return nil, fmt.Errorf("gagal menyimpan materi ke database: %w", err)
	}

	return materiList, nil
}