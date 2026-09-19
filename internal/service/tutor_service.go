package service

import (
	"fmt"
	"log"
	"time"

	"momo-be/internal/job"
	"momo-be/pkg/aiclient"
)

// JoinInfo adalah info auto-join onboarding (TANPA token).
type JoinInfo struct {
	SiswaID   uint   `json:"siswa_id"`
	KelasID   uint   `json:"kelas_id"`
	Nama      string `json:"nama"`
	KelasNama string `json:"kelas_nama"`
}

// ChatResult adalah response lengkap percakapan tutor.
type ChatResult struct {
	Balasan     string    `json:"balasan"`
	Fase        string    `json:"fase"`
	ExtractNama string    `json:"extract_nama"`
	ExtractKode string    `json:"extract_kode_kelas"`
	Join        *JoinInfo `json:"join,omitempty"`
	JoinError   string    `json:"join_error,omitempty"`
}

type TutorService struct {
	aiClient     *aiclient.Client
	jobRegistry  *job.Registry
	callbackURL  string
	siswaService *SiswaService
}

func NewTutorService(
	aiClient *aiclient.Client,
	jobRegistry *job.Registry,
	callbackURL string,
	siswaService *SiswaService,
) *TutorService {
	return &TutorService{
		aiClient:     aiClient,
		jobRegistry:  jobRegistry,
		callbackURL:  callbackURL,
		siswaService: siswaService,
	}
}

func kodeKelasValid(kode string) bool {
	if len(kode) != 6 {
		return false
	}
	for _, c := range kode {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// ProcessTutor = SATU API percakapan + AUTO-JOIN onboarding.
// BACKEND PEMEGANG KEBENARAN: balasan final ditentukan hasil join,
// bukan klaim AI. AI boleh salah sangka, backend tidak.
func (s *TutorService) ProcessTutor(sessionID string, kelasNama string, pesanSiswa string) (*ChatResult, string, error) {
	jobID := job.GenerateID("tutor")

	s.jobRegistry.Register(jobID, job.JobTypeTutor, 0, "", sessionID)

	konteks := fmt.Sprintf("Kelas: %s | Session: %s", kelasNama, sessionID)

	// Padding pesan pendek agar tidak ditolak AI Service (min 10 karakter)
	teksKirim := pesanSiswa
	if len(pesanSiswa) < 10 {
		teksKirim = fmt.Sprintf("Siswa menjawab: %s", pesanSiswa)
	}

	err := s.aiClient.SubmitTutor(jobID, s.callbackURL, teksKirim, konteks, sessionID)
	if err != nil {
		log.Printf("[tutor] gagal kirim job %s: %v", jobID, err)
		return nil, jobID, fmt.Errorf("gagal menghubungi AI Service: %w", err)
	}

	log.Printf("[tutor] job %s (session %s) dikirim, menunggu reasoning AI...", jobID, sessionID)

	finished := s.jobRegistry.WaitSync(jobID, 90*time.Second)
	if finished == nil {
		return nil, jobID, fmt.Errorf("timeout menunggu balasan tutor (90 detik)")
	}
	if finished.Status == "failed" {
		return nil, jobID, fmt.Errorf("AI tutor gagal memproses: %s", finished.Error)
	}

	res, err := job.ParseTutorFull(finished.Hasil)
	if err != nil {
		return nil, jobID, fmt.Errorf("gagal parse balasan tutor: %w", err)
	}

	out := &ChatResult{
		Balasan:     res.Balasan,
		Fase:        res.Fase,
		ExtractNama: res.ExtractNama,
		ExtractKode: res.ExtractKode,
	}

	// ============ AUTO-JOIN: BACKEND YANG MEMUTUSKAN ============
	if out.ExtractNama != "" && out.ExtractKode != "" {
		if !kodeKelasValid(out.ExtractKode) {
			// Kode bukan 6 digit: override balasan, tetap di onboarding
			out.JoinError = "Kode kelas harus enam digit angka."
			out.Fase = "onboarding"
			out.Balasan = "Kode kelas terdiri dari enam digit angka. Coba sebutkan lagi kode kelas kamu ya."
			out.ExtractKode = ""
			log.Printf("[tutor] auto-join ditolak: format kode tidak valid")
		} else {
			siswa, _, errJoin := s.siswaService.JoinSiswa(out.ExtractKode, out.ExtractNama)
			if errJoin != nil {
				// KODE TIDAK DITEMUKAN: override klaim sukses AI dengan pesan error.
				// Ini fix bug "kode acak bisa masuk": yang di-speak sekarang error, bukan "terhubung".
				out.JoinError = errJoin.Error()
				out.Fase = "onboarding"
				out.Balasan = fmt.Sprintf("Maaf, kode kelas %s tidak ditemukan. Coba sebutkan lagi kode kelas kamu ya.", out.ExtractKode)
				out.ExtractKode = ""
				log.Printf("[tutor] auto-join GAGAL (kode tidak ditemukan): %v", errJoin)
			} else {
				// SUKSES: ikat session + konfirmasi resmi dengan nama kelas
				if errLink := s.siswaService.LinkSession(siswa.ID, sessionID); errLink != nil {
					log.Printf("[tutor] warning: gagal ikat session: %v", errLink)
				}
				namaKelas, _ := s.siswaService.NamaKelasByID(siswa.KelasID)
				out.Join = &JoinInfo{
					SiswaID:   siswa.ID,
					KelasID:   siswa.KelasID,
					Nama:      siswa.Nama,
					KelasNama: namaKelas,
				}
				out.Fase = "belajar"
				out.Balasan = fmt.Sprintf("%s Kamu sekarang resmi masuk kelas %s. Selamat belajar, %s!", out.Balasan, namaKelas, siswa.Nama)
				log.Printf("[tutor] auto-join SUKSES: siswa %d (%s) kelas %d (%s), session terikat", siswa.ID, siswa.Nama, siswa.KelasID, namaKelas)
			}
		}
	}

	log.Printf("[tutor] job %s selesai: fase=%s balasan=%d karakter join=%v join_error=%q", jobID, out.Fase, len(out.Balasan), out.Join != nil, out.JoinError)
	return out, jobID, nil
}