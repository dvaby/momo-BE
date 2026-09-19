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

// ProcessTutor = SATU API percakapan: kirim pesan ke AI, tunggu reasoning,
// return balasan teks. Pola sync-via-callback (sama seperti submit-jawaban).
func (s *TutorService) ProcessTutor(siswaID uint, kelasNama string, pesanSiswa string) (string, string, error) {
	jobID := job.GenerateID("tutor")
	sessionID := fmt.Sprintf("siswa-%d", siswaID) // key memory percakapan di AI Service

	s.jobRegistry.Register(jobID, job.JobTypeTutor, 0, "", strconv.FormatUint(uint64(siswaID), 10))

	konteks := fmt.Sprintf("Kelas: %s | Siswa ID: %d", kelasNama, siswaID)

	err := s.aiClient.SubmitTutor(jobID, s.callbackURL, pesanSiswa, konteks, sessionID)
	if err != nil {
		log.Printf("[tutor] gagal kirim job %s: %v", jobID, err)
		return "", jobID, fmt.Errorf("gagal menghubungi AI Service: %w", err)
	}

	log.Printf("[tutor] job %s (session %s) dikirim, menunggu reasoning AI...", jobID, sessionID)

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