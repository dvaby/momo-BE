package service

import (
	"fmt"
	"log"
	"strings"

	"momo-be/internal/model"
	"momo-be/internal/repository"
	"momo-be/pkg/aiclient"
	"momo-be/pkg/pdfworker"
	"momo-be/pkg/textutil"
)

type SoalService struct {
	repo      *repository.SoalRepository
	kelasRepo *repository.KelasRepository
	modulRepo repository.ModulRepository // Menggunakan interface langsung (tanpa *)
	aiClient  *aiclient.Client
}

func NewSoalService(
	repo *repository.SoalRepository,
	kelasRepo *repository.KelasRepository,
	modulRepo repository.ModulRepository, // Tanpa *
	aiClient *aiclient.Client,
) *SoalService {
	return &SoalService{
		repo:      repo,
		kelasRepo: kelasRepo,
		modulRepo: modulRepo,
		aiClient:  aiClient,
	}
}

func (s *SoalService) ValidateModulOwnership(modulID uint, guruID uint) error {
	_, err := s.modulRepo.FindByIDAndGuruID(modulID, guruID)
	if err != nil {
		return fmt.Errorf("akses ditolak: modul tidak ditemukan atau bukan milik Anda")
	}
	return nil
}

func (s *SoalService) ProcessAndSaveSoal(modulID uint, jenis model.JenisSoal, pdfFilePath string, guruID uint) ([]model.Soal, error) {
	// Validasi kepemilikan modul
	if err := s.ValidateModulOwnership(modulID, guruID); err != nil {
		return nil, err
	}

	teksMentah, err := pdfworker.ExtractText(pdfFilePath)
	if err != nil {
		return nil, fmt.Errorf("gagal ekstrak PDF: %w", err)
	}

	// Chunking teks panjang (2000 karakter per chunk, 200 karakter overlap)
	chunks := textutil.ChunkText(teksMentah, 2000, 200)

	var soalList []model.Soal

	for i, chunk := range chunks {
		log.Printf("[soal] memproses chunk %d/%d (%d karakter)", i+1, len(chunks), len(chunk))

		aiResponse, err := s.aiClient.ProcessText("soal", chunk)
		if err != nil {
			log.Printf("[soal] warning: chunk %d gagal: %v", i+1, err)
			continue // skip chunk yang gagal
		}

		if !aiResponse.Success {
			log.Printf("[soal] warning: chunk %d tidak sukses: %s", i+1, aiResponse.Message)
			continue
		}

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
	}

	if len(soalList) == 0 {
		return nil, fmt.Errorf("AI Service tidak menemukan soal pilihan ganda yang valid dari PDF ini")
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

// GetByModulIDAndJenis mengambil daftar soal milik modul (dengan cek kepemilikan guru)
func (s *SoalService) GetByModulIDAndJenis(modulID uint, guruID uint, jenis model.JenisSoal) ([]model.Soal, error) {
	if err := s.ValidateModulOwnership(modulID, guruID); err != nil {
		return nil, err
	}
	return s.repo.FindByModulAndJenis(modulID, jenis)
}

// CreateManual membuat soal tulis tangan (tanpa PDF/AI)
func (s *SoalService) CreateManual(modulID uint, guruID uint, jenis model.JenisSoal, pertanyaan, pilihanA, pilihanB, pilihanC, pilihanD, kunciJawaban string) (*model.Soal, error) {
	if err := s.ValidateModulOwnership(modulID, guruID); err != nil {
		return nil, err
	}
	if jenis != model.JenisSoalHarian && jenis != model.JenisSoalUTS && jenis != model.JenisSoalUAS {
		return nil, fmt.Errorf("jenis soal wajib salah satu dari: harian, uts, uas")
	}
	if strings.TrimSpace(pertanyaan) == "" {
		return nil, fmt.Errorf("pertanyaan wajib diisi")
	}
	if strings.TrimSpace(pilihanA) == "" || strings.TrimSpace(pilihanB) == "" ||
		strings.TrimSpace(pilihanC) == "" || strings.TrimSpace(pilihanD) == "" {
		return nil, fmt.Errorf("keempat pilihan jawaban (A/B/C/D) wajib diisi")
	}
	kunciJawaban = strings.ToUpper(strings.TrimSpace(kunciJawaban))
	if kunciJawaban != "A" && kunciJawaban != "B" && kunciJawaban != "C" && kunciJawaban != "D" {
		return nil, fmt.Errorf("kunci jawaban wajib salah satu dari: A, B, C, atau D")
	}

	soal := &model.Soal{
		ModulID:      modulID,
		Jenis:        jenis,
		Pertanyaan:   pertanyaan,
		PilihanA:     pilihanA,
		PilihanB:     pilihanB,
		PilihanC:     pilihanC,
		PilihanD:     pilihanD,
		KunciJawaban: kunciJawaban,
	}
	if err := s.repo.Create(soal); err != nil {
		return nil, fmt.Errorf("gagal menyimpan soal: %w", err)
	}
	return soal, nil
}

// validateSoalOwnership memastikan soal ada dan modul induknya milik guru ini
func (s *SoalService) validateSoalOwnership(soalID uint, guruID uint) (*model.Soal, error) {
	soal, err := s.repo.FindByID(soalID)
	if err != nil {
		return nil, fmt.Errorf("soal tidak ditemukan")
	}
	// Cek kepemilikan modul (asumsi SoalService punya akses ke modulRepo)
	if _, err := s.modulRepo.FindByIDAndGuruID(soal.ModulID, guruID); err != nil {
		return nil, fmt.Errorf("akses ditolak: soal ini bukan milik Anda")
	}
	return soal, nil
}

// Update mengubah pertanyaan/pilihan/kunci jawaban/jenis soal
func (s *SoalService) Update(soalID uint, guruID uint, pertanyaan, pilihanA, pilihanB, pilihanC, pilihanD, kunciJawaban string, jenis model.JenisSoal) (*model.Soal, error) {
	soal, err := s.validateSoalOwnership(soalID, guruID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(pertanyaan) == "" {
		return nil, fmt.Errorf("pertanyaan wajib diisi")
	}
	if strings.TrimSpace(pilihanA) == "" || strings.TrimSpace(pilihanB) == "" ||
		strings.TrimSpace(pilihanC) == "" || strings.TrimSpace(pilihanD) == "" {
		return nil, fmt.Errorf("keempat pilihan jawaban (A/B/C/D) wajib diisi")
	}
	kunciJawaban = strings.ToUpper(strings.TrimSpace(kunciJawaban))
	if kunciJawaban != "A" && kunciJawaban != "B" && kunciJawaban != "C" && kunciJawaban != "D" {
		return nil, fmt.Errorf("kunci jawaban wajib salah satu dari: A, B, C, atau D")
	}
	if jenis != "" && jenis != model.JenisSoalHarian && jenis != model.JenisSoalUTS && jenis != model.JenisSoalUAS {
		return nil, fmt.Errorf("jenis soal wajib salah satu dari: harian, uts, uas")
	}

	soal.Pertanyaan = pertanyaan
	soal.PilihanA = pilihanA
	soal.PilihanB = pilihanB
	soal.PilihanC = pilihanC
	soal.PilihanD = pilihanD
	soal.KunciJawaban = kunciJawaban
	if jenis != "" {
		soal.Jenis = jenis
	}
	if err := s.repo.Update(soal); err != nil {
		return nil, fmt.Errorf("gagal memperbarui soal: %w", err)
	}
	return soal, nil
}

// Delete menghapus satu soal
func (s *SoalService) Delete(soalID uint, guruID uint) error {
	soal, err := s.validateSoalOwnership(soalID, guruID)
	if err != nil {
		return err
	}
	return s.repo.Delete(soal.ID)
}
