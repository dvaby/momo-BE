package service

import (
	"fmt"

	"momo-be/internal/model"
	"momo-be/internal/repository"
	"momo-be/pkg/aiclient"
)

type JawabanSiswaService struct {
	repo      *repository.JawabanSiswaRepository
	soalRepo  *repository.SoalRepository
	siswaRepo *repository.SiswaRepository
	kelasRepo *repository.KelasRepository
	aiClient  *aiclient.Client
}

func NewJawabanSiswaService(
	repo *repository.JawabanSiswaRepository,
	soalRepo *repository.SoalRepository,
	siswaRepo *repository.SiswaRepository,
	kelasRepo *repository.KelasRepository,
	aiClient *aiclient.Client,
) *JawabanSiswaService {
	return &JawabanSiswaService{
		repo:      repo,
		soalRepo:  soalRepo,
		siswaRepo: siswaRepo,
		kelasRepo: kelasRepo,
		aiClient:  aiClient,
	}
}

func (s *JawabanSiswaService) SubmitJawaban(siswaID uint, soalID uint, jawabanMentah string) (*model.JawabanSiswa, error) {
	// 1. Ambil data profil siswa untuk mendapatkan kelas_id
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

	// 5. Evaluasi jawaban via AI Service
	evalReq := aiclient.EvaluateRequest{
		Pertanyaan:         soal.Pertanyaan,
		PilihanA:           soal.PilihanA,
		PilihanB:           soal.PilihanB,
		PilihanC:           soal.PilihanC,
		PilihanD:           soal.PilihanD,
		KunciJawaban:       soal.KunciJawaban,
		JawabanSiswaMentah: jawabanMentah,
	}

	aiResponse, err := s.aiClient.EvaluateAnswer(evalReq)
	if err != nil {
		return nil, fmt.Errorf("gagal menghubungi AI Service: %w", err)
	}

	if !aiResponse.Success {
		errMsg := "AI Service gagal mengevaluasi jawaban"
		if aiResponse.Message != "" {
			errMsg = aiResponse.Message
		}
		return nil, fmt.Errorf(errMsg)
	}

	// 6. Simpan baris jawaban siswa
	jawaban := &model.JawabanSiswa{
		SiswaID:           siswaID,
		SoalID:            soalID,
		JawabanMentah:     jawabanMentah,
		JawabanTerdeteksi: aiResponse.Data.JawabanTerdeteksi,
		Benar:             aiResponse.Data.Benar,
		Feedback:          aiResponse.Data.Feedback,
	}

	err = s.repo.Create(jawaban)
	if err != nil {
		return nil, fmt.Errorf("gagal menyimpan jawaban: %w", err)
	}

	return jawaban, nil
}