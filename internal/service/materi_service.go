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

type MateriService struct {
	repo        *repository.MateriRepository
	modulRepo   repository.ModulRepository
	aiClient    *aiclient.Client
	jobRegistry *job.Registry // BARU
	callbackURL string        // BARU
}

func NewMateriService(
	repo *repository.MateriRepository,
	modulRepo repository.ModulRepository,
	aiClient *aiclient.Client,
	jobRegistry *job.Registry, // BARU
	callbackURL string, // BARU
) *MateriService {
	return &MateriService{
		repo:        repo,
		modulRepo:   modulRepo,
		aiClient:    aiClient,
		jobRegistry: jobRegistry,
		callbackURL: callbackURL,
	}
}

func (s *MateriService) ValidateModulOwnership(modulID uint, guruID uint) error {
	_, err := s.modulRepo.FindByIDAndGuruID(modulID, guruID)
	if err != nil {
		return fmt.Errorf("akses ditolak: modul tidak ditemukan atau bukan milik Anda")
	}
	return nil
}

func (s *MateriService) rakitKonteks(modulID uint) string {
	modul, err := s.modulRepo.FindByID(modulID)
	if err != nil {
		return fmt.Sprintf("Modul ID: %d", modulID)
	}
	return fmt.Sprintf("Modul: %s", modul.Nama)
}

func (s *MateriService) prosesSatuChunk(chunk string, chunkIdx, totalChunks int, konteks string) ([]aiclient.MateriItem, error) {
	jobID := job.GenerateID("materi")
	s.jobRegistry.Register(jobID, job.JobTypeMateri, 0, "", "")

	result, err := s.aiClient.ProcessTextWithJob("materi", chunk, jobID, s.callbackURL, konteks)
	if err != nil {
		return nil, fmt.Errorf("gagal menghubungi AI Service: %w", err)
	}

	if result.Ack != nil {
		log.Printf("[materi] chunk %d/%d: ack diterima, menunggu callback...", chunkIdx, totalChunks)
		finished := s.jobRegistry.WaitSync(jobID, 150*time.Second)
		if finished == nil {
			return nil, fmt.Errorf("timeout menunggu callback dari AI Service (150 detik)")
		}
		if finished.Status == "failed" {
			return nil, fmt.Errorf("AI Service gagal memproses chunk: %s", finished.Error)
		}
		materiItems, _, err := job.ParseProcessResult(finished.Hasil)
		if err != nil {
			return nil, fmt.Errorf("gagal parse hasil: %w", err)
		}
		return materiItems, nil
	}

	if result.ProcessResult != nil {
		if !result.ProcessResult.Success {
			return nil, fmt.Errorf("AI Service gagal: %s", result.ProcessResult.Message)
		}
		return result.ProcessResult.Data.Materi, nil
	}

	return nil, fmt.Errorf("response AI Service tidak dikenali")
}

func (s *MateriService) ProcessAndSaveMateri(modulID uint, pdfFilePath string) ([]model.Materi, error) {
	teksMentah, err := pdfworker.ExtractText(pdfFilePath)
	if err != nil {
		return nil, fmt.Errorf("gagal ekstrak PDF: %w", err)
	}

	konteks := s.rakitKonteks(modulID)
	chunks := textutil.ChunkText(teksMentah, 3000, 500)

	var materiList []model.Materi
	urutanCounter := 1
	chunkGagal := 0

	for i, chunk := range chunks {
		log.Printf("[materi] memproses chunk %d/%d (%d karakter)", i+1, len(chunks), len(chunk))

		materiItems, err := s.prosesSatuChunk(chunk, i+1, len(chunks), konteks)
		if err != nil {
			log.Printf("[materi] warning: chunk %d gagal: %v", i+1, err)
			chunkGagal++
			continue
		}

		for _, item := range materiItems {
			materiList = append(materiList, model.Materi{
				ModulID: modulID,
				Urutan:  urutanCounter,
				Judul:   item.Judul,
				Konten:  item.Konten,
			})
			urutanCounter++
		}
	}

	if len(materiList) == 0 {
		return nil, fmt.Errorf("AI Service tidak menemukan konten materi yang valid dari PDF ini — pastikan PDF berisi materi pembelajaran, bukan soal (chunk gagal: %d/%d)", chunkGagal, len(chunks))
	}

	if err := s.repo.CreateBatch(materiList); err != nil {
		return nil, fmt.Errorf("gagal menyimpan materi ke database: %w", err)
	}

	return materiList, nil
}

// --- Method-method di bawah TIDAK BERUBAH ---

func (s *MateriService) GetByModulID(modulID uint, guruID uint) ([]model.Materi, error) {
	if err := s.ValidateModulOwnership(modulID, guruID); err != nil {
		return nil, err
	}
	return s.repo.FindByModulID(modulID)
}

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
	materi := &model.Materi{ModulID: modulID, Urutan: urutan, Judul: judul, Konten: konten}
	if err := s.repo.Create(materi); err != nil {
		return nil, fmt.Errorf("gagal menyimpan materi: %w", err)
	}
	return materi, nil
}

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

func (s *MateriService) Delete(materiID uint, guruID uint) error {
	materi, err := s.validateMateriOwnership(materiID, guruID)
	if err != nil {
		return err
	}
	return s.repo.Delete(materi.ID)
}
