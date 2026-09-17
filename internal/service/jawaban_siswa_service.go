package service

import (
	"fmt"
	"log"
	"time"

	"momo-be/internal/job"
	"momo-be/internal/model"
	"momo-be/internal/repository"
	"momo-be/pkg/aiclient"
)

type JawabanSiswaService struct {
	repo        *repository.JawabanSiswaRepository
	soalRepo    *repository.SoalRepository
	siswaRepo   *repository.SiswaRepository
	kelasRepo   *repository.KelasRepository
	aiClient    *aiclient.Client
	jobRegistry *job.Registry // BARU
	callbackURL string        // BARU
}

func NewJawabanSiswaService(
	repo *repository.JawabanSiswaRepository,
	soalRepo *repository.SoalRepository,
	siswaRepo *repository.SiswaRepository,
	kelasRepo *repository.KelasRepository,
	aiClient *aiclient.Client,
	jobRegistry *job.Registry, // BARU
	callbackURL string, // BARU
) *JawabanSiswaService {
	return &JawabanSiswaService{
		repo:        repo,
		soalRepo:    soalRepo,
		siswaRepo:   siswaRepo,
		kelasRepo:   kelasRepo,
		aiClient:    aiClient,
		jobRegistry: jobRegistry,
		callbackURL: callbackURL,
	}
}

// rakitKonteks buat string konteks untuk AI Service (inferensi jenjang)
func (s *JawabanSiswaService) rakitKonteks(kelasID uint) string {
	kelas, err := s.kelasRepo.FindByID(kelasID)
	if err != nil {
		return fmt.Sprintf("Kelas ID: %d", kelasID)
	}
	return fmt.Sprintf("Kelas: %s (%s)", kelas.NamaKelas, kelas.MataPelajaran)
}

func (s *JawabanSiswaService) SubmitJawaban(siswaID uint, soalID uint, jawabanMentah string) (*model.JawabanSiswa, error) {
	// 1. Ambil data profil siswa
	siswa, err := s.siswaRepo.FindByID(siswaID)
	if err != nil {
		return nil, fmt.Errorf("data siswa tidak ditemukan: %w", err)
	}

	// 2. Ambil detail soal
	soal, err := s.soalRepo.FindByID(soalID)
	if err != nil {
		return nil, fmt.Errorf("soal dengan ID %d tidak ditemukan", soalID)
	}

	// 3. Verifikasi apakah modul dari soal ini ditugaskan ke kelas siswa
	allowed, err := s.kelasRepo.IsModulInKelas(siswa.KelasID, soal.ModulID)
	if err != nil {
		return nil, fmt.Errorf("gagal memverifikasi hak akses modul: %w", err)
	}
	if !allowed {
		return nil, fmt.Errorf("soal ini tidak ditugaskan untuk kelas Anda")
	}

	// 4. Aturan: soal UTS/UAS hanya boleh dijawab sekali per siswa
	if soal.Jenis == model.JenisSoalUTS || soal.Jenis == model.JenisSoalUAS {
		jawabanSebelumnya, err := s.repo.FindBySiswaIDAndSoalID(siswaID, soalID)
		if err == nil && jawabanSebelumnya != nil {
			return nil, fmt.Errorf("soal ini sudah pernah dijawab dan tidak bisa diulang")
		}
	}

	// 5. Evaluasi jawaban via AI Service dengan sync-via-callback
	konteks := s.rakitKonteks(siswa.KelasID)
	jobID := job.GenerateID("eval")
	s.jobRegistry.Register(jobID, job.JobTypeEvaluate, 0, "", "")

	evalReq := aiclient.EvaluateRequest{
		Pertanyaan:         soal.Pertanyaan,
		PilihanA:           soal.PilihanA,
		PilihanB:           soal.PilihanB,
		PilihanC:           soal.PilihanC,
		PilihanD:           soal.PilihanD,
		KunciJawaban:       soal.KunciJawaban,
		JawabanSiswaMentah: jawabanMentah,
		JobID:              jobID,
		CallbackURL:        s.callbackURL,
		Konteks:            konteks,
	}

	result, err := s.aiClient.EvaluateAnswerWithJob(evalReq)
	if err != nil {
		return nil, fmt.Errorf("gagal menghubungi AI Service: %w", err)
	}

	var jawabanTerdeteksi string
	var benar bool
	var feedback string

	if result.Ack != nil {
		log.Printf("[jawaban] ack diterima untuk soal %d, menunggu feedback...", soalID)
		finished := s.jobRegistry.WaitSync(jobID, 30*time.Second)
		if finished == nil {
			return nil, fmt.Errorf("timeout menunggu feedback dari AI Service (30 detik)")
		}
		if finished.Status == "failed" {
			return nil, fmt.Errorf("AI Service gagal mengevaluasi jawaban: %s", finished.Error)
		}
		jawabanTerdeteksi, benar, feedback, _, err = job.ParseEvaluateResult(finished.Hasil)
		if err != nil {
			return nil, fmt.Errorf("gagal parse feedback: %w", err)
		}
	} else if result.EvalResult != nil {
		// Pola lama (pola sync langsung)
		if !result.EvalResult.Success {
			errMsg := "AI Service gagal mengevaluasi jawaban"
			if result.EvalResult.Message != "" {
				errMsg = result.EvalResult.Message
			}
			return nil, fmt.Errorf(errMsg)
		}
		jawabanTerdeteksi = result.EvalResult.Data.JawabanTerdeteksi
		benar = result.EvalResult.Data.Benar
		feedback = result.EvalResult.Data.Feedback
	} else {
		return nil, fmt.Errorf("response AI Service tidak dikenali")
	}

	// 6. Simpan jawaban siswa
	jawaban := &model.JawabanSiswa{
		SiswaID:           siswaID,
		SoalID:            soalID,
		JawabanMentah:     jawabanMentah,
		JawabanTerdeteksi: jawabanTerdeteksi,
		Benar:             benar,
		Feedback:          feedback,
	}

	if err := s.repo.Create(jawaban); err != nil {
		return nil, fmt.Errorf("gagal menyimpan jawaban: %w", err)
	}

	return jawaban, nil
}
