package service

import (
	"fmt"

	"momo-be/internal/model"
	"momo-be/internal/repository"
	"momo-be/pkg/aiclient"
	"momo-be/pkg/pdfworker"
)

type SoalService struct {
	repo      *repository.SoalRepository
	kelasRepo *repository.KelasRepository
	aiClient  *aiclient.Client
}

func NewSoalService(repo *repository.SoalRepository, kelasRepo *repository.KelasRepository, aiClient *aiclient.Client) *SoalService {
	return &SoalService{
		repo:      repo,
		kelasRepo: kelasRepo,
		aiClient:  aiClient,
	}
}

func (s *SoalService) ProcessAndSaveSoal(modulID uint, jenis model.JenisSoal, pdfFilePath string) ([]model.Soal, error) {
	teksMentah, err := pdfworker.ExtractText(pdfFilePath)
	if err != nil {
		return nil, fmt.Errorf("gagal ekstrak PDF: %w", err)
	}

	aiResponse, err := s.aiClient.ProcessText("soal", teksMentah)
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

	var soalList []model.Soal
	for _, item := range aiResponse.Data.Soal {
		soalList = append(soalList, model.Soal{
			ModulID:      modulID,
			Jenis:        jenis,
			Pertanyaan:   item.Pertanyaan,
			PilihanA:     item.PilihanA,
			PilihanB:     item.PilihanB,
			PilihanC:     item.PilihanC,
			PilihanD:     item.PilihanD,
			KunciJawaban: item.KunciJawaban,
		})
	}

	if len(soalList) == 0 {
		return nil, fmt.Errorf("AI Service tidak menemukan konten soal yang valid dari PDF ini — pastikan PDF berisi soal, bukan materi")
	}

	err = s.repo.CreateBatch(soalList)
	if err != nil {
		return nil, fmt.Errorf("gagal menyimpan soal ke database: %w", err)
	}

	return soalList, nil
}

func (s *SoalService) GetSoalByModulAndJenisForSiswa(modulID uint, kelasID uint, jenis model.JenisSoal) ([]model.Soal, error) {
	allowed, err := s.kelasRepo.IsModulInKelas(kelasID, modulID)
	if err != nil {
		return nil, fmt.Errorf("gagal memverifikasi akses modul: %w", err)
	}
	if !allowed {
		return nil, fmt.Errorf("modul ini tidak ditugaskan untuk kelas Anda")
	}

	return s.repo.FindByModulAndJenis(modulID, jenis)
}