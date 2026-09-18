package service

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"momo-be/internal/job"
	"momo-be/internal/model"
	"momo-be/internal/repository"
	"momo-be/pkg/aiclient"
	"momo-be/pkg/pdfworker"
	"momo-be/pkg/textutil"
)

type SoalService struct {
	repo        *repository.SoalRepository
	kelasRepo   *repository.KelasRepository
	modulRepo   repository.ModulRepository
	aiClient    *aiclient.Client
	jobRegistry *job.Registry
	callbackURL string
}

func NewSoalService(
	repo *repository.SoalRepository,
	kelasRepo *repository.KelasRepository,
	modulRepo repository.ModulRepository,
	aiClient *aiclient.Client,
	jobRegistry *job.Registry,
	callbackURL string,
) *SoalService {
	return &SoalService{
		repo:        repo,
		kelasRepo:   kelasRepo,
		modulRepo:   modulRepo,
		aiClient:    aiClient,
		jobRegistry: jobRegistry,
		callbackURL: callbackURL,
	}
}

func (s *SoalService) ValidateModulOwnership(modulID uint, guruID uint) error {
	_, err := s.modulRepo.FindByIDAndGuruID(modulID, guruID)
	if err != nil {
		return fmt.Errorf("akses ditolak: modul tidak ditemukan atau bukan milik Anda")
	}
	return nil
}

func (s *SoalService) rakitKonteks(modulID uint) string {
	modul, err := s.modulRepo.FindByID(modulID)
	if err != nil {
		return fmt.Sprintf("Modul ID: %d", modulID)
	}
	return fmt.Sprintf("Modul: %s", modul.Nama)
}

func (s *SoalService) prosesSatuChunk(chunk string, chunkIdx, totalChunks int, konteks string) ([]aiclient.SoalItem, error) {
	jobID := job.GenerateID("soal")
	s.jobRegistry.Register(jobID, job.JobTypeSoal, 0, "", "")

	result, err := s.aiClient.ProcessTextWithJob("soal", chunk, jobID, s.callbackURL, konteks)
	if err != nil {
		return nil, fmt.Errorf("gagal menghubungi AI Service: %w", err)
	}

	if result.Ack != nil {
		log.Printf("[soal] chunk %d/%d: ack diterima, menunggu callback...", chunkIdx, totalChunks)
		finished := s.jobRegistry.WaitSync(jobID, 150*time.Second)
		if finished == nil {
			return nil, fmt.Errorf("timeout menunggu callback dari AI Service (150 detik)")
		}
		if finished.Status == "failed" {
			return nil, fmt.Errorf("AI Service gagal memproses chunk: %s", finished.Error)
		}
		_, soalItems, err := job.ParseProcessResult(finished.Hasil)
		if err != nil {
			return nil, fmt.Errorf("gagal parse hasil: %w", err)
		}
		return soalItems, nil
	}

	if result.ProcessResult != nil {
		if !result.ProcessResult.Success {
			return nil, fmt.Errorf("AI Service gagal: %s", result.ProcessResult.Message)
		}
		return result.ProcessResult.Data.Soal, nil
	}

	return nil, fmt.Errorf("response AI Service tidak dikenali")
}

// processChunkResult adalah hasil dari satu chunk (untuk aggregation parallel)
type processChunkResult struct {
	chunkIdx int
	soal     []model.Soal
	err      error
}

func (s *SoalService) ProcessAndSaveSoal(modulID uint, jenis model.JenisSoal, pdfFilePath string, guruID uint) ([]model.Soal, error) {
	if err := s.ValidateModulOwnership(modulID, guruID); err != nil {
		return nil, err
	}

	teksMentah, err := pdfworker.ExtractText(pdfFilePath)
	if err != nil {
		return nil, fmt.Errorf("gagal ekstrak PDF: %w", err)
	}

	konteks := s.rakitKonteks(modulID)
	
	// SMART CHUNKING: distribute karakter lebih merata untuk optimal parallel
	// Target: 4 chunks, min 1500 chars, max 2500 chars per chunk
	chunks := textutil.SmartChunkText(teksMentah, 4, 1500, 2500)
	
	log.Printf("[soal] smart chunking: %d chunks, sizes: %v", len(chunks), getChunkSizes(chunks))

	// PARALLEL: spawn goroutine per chunk
	results := make(chan processChunkResult, len(chunks))
	var wg sync.WaitGroup

	for i, chunk := range chunks {
		wg.Add(1)
		go func(idx int, chunkText string) {
			defer wg.Done()

			log.Printf("[soal] [goroutine %d/%d] memproses chunk (%d karakter)", idx+1, len(chunks), len(chunkText))

			soalItems, err := s.prosesSatuChunk(chunkText, idx+1, len(chunks), konteks)
			if err != nil {
				log.Printf("[soal] [goroutine %d] gagal: %v", idx+1, err)
				results <- processChunkResult{chunkIdx: idx + 1, err: err}
				return
			}

			var soalList []model.Soal
			for _, item := range soalItems {
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

			results <- processChunkResult{chunkIdx: idx + 1, soal: soalList}
		}(i, chunk)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var soalList []model.Soal
	chunkGagal := 0

	for res := range results {
		if res.err != nil {
			chunkGagal++
			continue
		}
		soalList = append(soalList, res.soal...)
	}

	if len(soalList) == 0 {
		return nil, fmt.Errorf("AI Service tidak menemukan soal pilihan ganda yang valid dari PDF ini (chunk gagal: %d/%d)", chunkGagal, len(chunks))
	}

	if err := s.repo.CreateBatch(soalList); err != nil {
		return nil, fmt.Errorf("gagal menyimpan soal ke database: %w", err)
	}

	log.Printf("[soal] parallel processing selesai: %d soal dari %d chunks (gagal: %d)", len(soalList), len(chunks), chunkGagal)
	return soalList, nil
}

// Helper function untuk logging chunk sizes
func getChunkSizes(chunks []string) []int {
	sizes := make([]int, len(chunks))
	for i, chunk := range chunks {
		sizes[i] = len(chunk)
	}
	return sizes
}

// --- Method-method di bawah TIDAK BERUBAH ---

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

func (s *SoalService) GetByModulIDAndJenis(modulID uint, guruID uint, jenis model.JenisSoal) ([]model.Soal, error) {
	if err := s.ValidateModulOwnership(modulID, guruID); err != nil {
		return nil, err
	}
	return s.repo.FindByModulAndJenis(modulID, jenis)
}

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

func (s *SoalService) validateSoalOwnership(soalID uint, guruID uint) (*model.Soal, error) {
	soal, err := s.repo.FindByID(soalID)
	if err != nil {
		return nil, fmt.Errorf("soal tidak ditemukan")
	}
	if _, err := s.modulRepo.FindByIDAndGuruID(soal.ModulID, guruID); err != nil {
		return nil, fmt.Errorf("akses ditolak: soal ini bukan milik Anda")
	}
	return soal, nil
}

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

func (s *SoalService) Delete(soalID uint, guruID uint) error {
	soal, err := s.validateSoalOwnership(soalID, guruID)
	if err != nil {
		return err
	}
	return s.repo.Delete(soal.ID)
}