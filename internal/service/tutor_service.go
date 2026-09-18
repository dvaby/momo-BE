package service

import (
	"fmt"
	"log"
	"strconv"

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

// SubmitTutorRequest kirim pesan ke AI Service (tipe=tutor) dengan ACK pattern.
// Tidak tunggu hasil — hasil dikirim via callback, lalu broadcast via event 'tutor-reply'.
// SessionID di registry dipakai untuk menyimpan siswa_id sebagai string.
func (s *TutorService) SubmitTutorRequest(siswaID uint, kelasNama string, pesanSiswa string) (string, error) {
	jobID := job.GenerateID("tutor")

	// Register job, simpan siswa_id di SessionID (sebagai string) untuk routing tutor-reply
	s.jobRegistry.Register(jobID, job.JobTypeTutor, 0, "", strconv.FormatUint(uint64(siswaID), 10))

	konteks := fmt.Sprintf("Kelas: %s | Siswa ID: %d", kelasNama, siswaID)

	// Kirim ke AI Service (fire and forget, AI akan callback)
	err := s.aiClient.SubmitTutor(jobID, s.callbackURL, pesanSiswa, konteks)
	if err != nil {
		// Cleanup job kalau gagal dikirim
		log.Printf("[tutor] gagal kirim job %s: %v", jobID, err)
		return "", fmt.Errorf("gagal menghubungi AI Service: %w", err)
	}

	log.Printf("[tutor] job %s dikirim ke AI untuk siswa %d, menunggu callback...", jobID, siswaID)
	return jobID, nil
}