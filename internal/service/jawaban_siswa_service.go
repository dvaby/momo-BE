package service

import (
	"fmt"

	"momo-be/internal/model"
	"momo-be/internal/repository"
	"momo-be/pkg/aiclient"
)

type JawabanSiswaService struct {
	repo     *repository.JawabanSiswaRepository
	soalRepo *repository.SoalRepository
	aiClient *aiclient.Client
}

func NewJawabanSiswaService(repo *repository.JawabanSiswaRepository, soalRepo *repository.SoalRepository, aiClient *aiclient.Client) *JawabanSiswaService {
	return &JawabanSiswaService{repo: repo, soalRepo: soalRepo, aiClient: aiClient}
}

// SubmitJawaban adalah alur inti: siswa menjawab 1 soal lewat suara ->
// BE ambil detail soal (termasuk kunci jawaban) -> BE kirim ke AI Service
// untuk dinilai -> BE simpan hasilnya -> balikin hasil evaluasi ke handler
// (untuk dibacakan lewat TTS).
func (s *JawabanSiswaService) SubmitJawaban(siswaID uint, soalID uint, jawabanMentah string) (*model.JawabanSiswa, error) {
	soal, err := s.soalRepo.FindByID(soalID)
	if err != nil {
		return nil, fmt.Errorf("soal dengan ID %d tidak ditemukan", soalID)
	}

	// Aturan: soal uts/uas hanya boleh dijawab sekali. Soal harian boleh
	// dijawab berkali-kali (misal karena STT salah tangkap sebelumnya).
	if soal.Jenis == model.JenisSoalUTS || soal.Jenis == model.JenisSoalUAS {
		jawabanSebelumnya, err := s.repo.FindBySiswaIDAndSoalID(siswaID, soalID)
		if err == nil && jawabanSebelumnya != nil {
			return nil, fmt.Errorf("soal ini sudah pernah dijawab dan tidak bisa diulang")
		}
	}

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