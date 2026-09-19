package service

import (
	"fmt"
	"log"
	"time"

	"momo-be/internal/job"
	"momo-be/pkg/aiclient"
)

// JoinInfo adalah info auto-join onboarding (TANPA token — FE tidak butuh token).
type JoinInfo struct {
	SiswaID uint   `json:"siswa_id"`
	KelasID uint   `json:"kelas_id"`
	Nama    string `json:"nama"`
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

func (s *TutorService) ProcessTutor(sessionID string, kelasNama string, pesanSiswa string) (*ChatResult, string, error) {
	jobID := job.GenerateID("tutor")

	s.jobRegistry.Register(jobID, job.JobTypeTutor, 0, "", sessionID)

	konteks := fmt.Sprintf("Kelas: %s | Session: %s", kelasNama, sessionID)

	// PATCH: padding pesan agar tidak ditolak AI Service (min 10 karakter)
	// Kalau pesan pendek seperti "Halo" / "Ya" / "Benar", tambahkan prefix konteks
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

	if out.ExtractNama != "" && out.ExtractKode != "" {
		if !kodeKelasValid(out.ExtractKode) {
			out.JoinError = "Kode kelas harus enam digit angka. Sebutkan ulang kode kelas kamu ya."
		} else {
			siswa, _, errJoin := s.siswaService.JoinSiswa(out.ExtractKode, out.ExtractNama)
			if errJoin != nil {
				out.JoinError = errJoin.Error()
				log.Printf("[tutor] auto-join gagal: %v", errJoin)
			} else {
				if errLink := s.siswaService.LinkSession(siswa.ID, sessionID); errLink != nil {
					log.Printf("[tutor] warning: gagal ikat session: %v", errLink)
				}
				out.Join = &JoinInfo{SiswaID: siswa.ID, KelasID: siswa.KelasID, Nama: siswa.Nama}
				log.Printf("[tutor] auto-join sukses: siswa %d (%s) kelas %d, session terikat", siswa.ID, siswa.Nama, siswa.KelasID)
			}
		}
	}

	log.Printf("[tutor] job %s selesai: fase=%s balasan=%d karakter join=%v", jobID, out.Fase, len(out.Balasan), out.Join != nil)
	return out, jobID, nil
}