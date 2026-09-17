package service

import (
	"fmt"
	"log"
	"strings"
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
	jobRegistry *job.Registry // BARU
	callbackURL string        // BARU
}

func NewSoalService(
	repo *repository.SoalRepository,
	kelasRepo *repository.KelasRepository,
	modulRepo repository.ModulRepository,
	aiClient *aiclient.Client,
	jobRegistry *job.Registry, // BARU
	callbackURL string, // BARU
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

// rakitKonteks buat string konteks untuk AI Service (inferensi jenjang)
func (s *SoalService) rakitKonteks(modulID uint) string {
	modul, err := s.modulRepo.FindByID(modulID)
	if err != nil {
		return fmt.Sprintf("Modul ID: %d", modulID)
	}
	return fmt.Sprintf("Modul: %s", modul.Nama)
}

// prosesSatuChunk kirim 1 chunk ke AI Service via sync-via-callback pattern.
// Return list soal yang berhasil diekstrak dari chunk ini.
func (s *SoalService) prosesSatuChunk(chunk string, chunkIdx, totalChunks int, konteks string) ([]aiclient.SoalItem, error) {
	jobID := job.GenerateID("soal")
	s.jobRegistry.Register(jobID, job.JobTypeSoal, 0, "", "")

	result, err := s.aiClient.ProcessTextWithJob("soal", chunk, jobID, s.callbackURL, konteks)
	if err != nil {
		return nil, fmt.Errorf("gagal menghubungi AI Service: %w", err)
	}

	// Pola baru (ack) — tunggu callback
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

	// Pola lama (response langsung) — fallback untuk AI Service versi lama
	if result.ProcessResult != nil {
		if !result.ProcessResult.Success {
			return nil, fmt.Errorf("AI Service gagal: %s", result.ProcessResult.Message)
		}
		return result.ProcessResult.Data.Soal, nil
	}

	return nil, fmt.Errorf("response AI Service tidak dikenali")
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
	chunks := textutil.ChunkText(teksMentah, 2000, 200)

	var soalList []model.Soal
	chunkGagal := 0

	for i, chunk := range chunks {
		log.Printf("[soal] memproses chunk %d/%d (%d karakter)", i+1, len(chunks), len(chunk))

		soalItems, err := s.prosesSatuChunk(chunk, i+1, len(chunks), konteks)
		if err != nil {
			log.Printf("[soal] warning: chunk %d gagal: %v", i+1, err)
			chunkGagal++
			continue
		}

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
	}

	if len(soalList) == 0 {
		return nil, fmt.Errorf("AI Service tidak menemukan soal pilihan ganda yang valid dari PDF ini (chunk gagal: %d/%d)", chunkGagal, len(chunks))
	}

	if err := s.repo.CreateBatch(soalList); err != nil {
		return nil, fmt.Errorf("gagal menyimpan soal ke database: %w", err)
	}

	return soalList, nil
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
		ModulID: modulID, Jenis: jenis,
		Pertanyaan: pertanyaan, PilihanA: pilihanA, PilihanB: pilihanB,
		PilihanC: pilihanC, PilihanD: pilihanD, KunciJawaban: kunciJawaban,
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
