package service

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"momo-be/internal/job"
	"momo-be/pkg/aiclient"
)

type TutorService struct {
	aiClient    *aiclient.Client
	jobRegistry *job.Registry
	callbackURL string
}

func NewTutorService(
	aiClient *aiclient.Client,
	jobRegistry *job.Registry,
	callbackURL string,
) *TutorService {
	return &TutorService{
		aiClient:    aiClient,
		jobRegistry: jobRegistry,
		callbackURL: callbackURL,
	}
}

// ProcessTutor mengirim pesan siswa ke AI Service dan MENUNGGU balasan
// (sync-via-callback, pola sama seperti submit-jawaban).
// Return: balasan teks, jobID, error.
func (s *TutorService) ProcessTutor(siswaID uint, kelasNama string, pesanSiswa string) (string, string, error) {
	jobID := job.GenerateID("tutor")

	// Register job; siswa_id disimpan di SessionID untuk routing event stream
	s.jobRegistry.Register(jobID, job.JobTypeTutor, 0, "", strconv.FormatUint(uint64(siswaID), 10))

	konteks := fmt.Sprintf("Kelas: %s | Siswa ID: %d", kelasNama, siswaID)

	err := s.aiClient.SubmitTutor(jobID, s.callbackURL, pesanSiswa, konteks)
	if err != nil {
		log.Printf("[tutor] gagal kirim job %s: %v", jobID, err)
		return "", jobID, fmt.Errorf("gagal menghubungi AI Service: %w", err)
	}

	log.Printf("[tutor] job %s dikirim, menunggu callback (max 90 detik)...", jobID)

	finished := s.jobRegistry.WaitSync(jobID, 90*time.Second)
	if finished == nil {
		return "", jobID, fmt.Errorf("timeout menunggu balasan tutor (90 detik)")
	}
	if finished.Status == "failed" {
		return "", jobID, fmt.Errorf("AI tutor gagal memproses: %s", finished.Error)
	}

	balasan, err := job.ParseTutorResult(finished.Hasil)
	if err != nil {
		return "", jobID, fmt.Errorf("gagal parse balasan tutor: %w", err)
	}

	log.Printf("[tutor] job %s selesai: balasan %d karakter", jobID, len(balasan))
	return balasan, jobID, nil
}