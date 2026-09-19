package service

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
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

	mu                sync.Mutex
	sessionLastCode   map[string]string // session -> kode 6 digit terakhir yang DICAPAI siswa
	sessionFailedCode map[string]string // session -> kode yang gagal join (jangan di-retry buta)
}

var sixDigits = regexp.MustCompile(`\b\d{6}\b`)

func NewTutorService(
	aiClient *aiclient.Client,
	jobRegistry *job.Registry,
	callbackURL string,
	siswaService *SiswaService,
) *TutorService {
	return &TutorService{
		aiClient:          aiClient,
		jobRegistry:       jobRegistry,
		callbackURL:       callbackURL,
		siswaService:      siswaService,
		sessionLastCode:   make(map[string]string),
		sessionFailedCode: make(map[string]string),
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

// sanitizeNama membersihkan artefak padding backend dari nama hasil extract AI.
// Contoh kotor: "Siswa menjawab: halo oke" -> "halo oke"
func sanitizeNama(nama string) string {
	n := strings.TrimSpace(nama)
	if idx := strings.Index(n, "menjawab:"); idx >= 0 {
		n = n[idx+len("menjawab:"):]
	}
	return strings.TrimSpace(n)
}

// ProcessTutor = SATU API percakapan + AUTO-JOIN onboarding.
// Backend pemegang kebenaran: balasan final ditentukan hasil join, bukan klaim AI.
func (s *TutorService) ProcessTutor(sessionID string, kelasNama string, pesanSiswa string) (*ChatResult, string, error) {
	jobID := job.GenerateID("tutor")

	s.jobRegistry.Register(jobID, job.JobTypeTutor, 0, "", sessionID)

	// REKAM kode 6 digit yang benar-benar disebut siswa di giliran ini.
	// Ini sumber kebenaran utama (mengatasi extract AI yang lengket/salah).
	if m := sixDigits.FindString(pesanSiswa); m != "" {
		s.mu.Lock()
		s.sessionLastCode[sessionID] = m
		s.mu.Unlock()
	}

	konteks := fmt.Sprintf("Kelas: %s | Session: %s", kelasNama, sessionID)

	// Padding pesan pendek (kalau AI Service masih min_length 10)
	teksKirim := pesanSiswa
	if len(pesanSiswa) < 10 {
		teksKirim = "Siswa menjawab: " + pesanSiswa
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
		ExtractNama: sanitizeNama(res.ExtractNama),
		ExtractKode: res.ExtractKode,
	}

	// AUTO-JOIN hanya saat AI menyatakan extract lengkap
	if out.ExtractNama != "" && out.ExtractKode != "" {
		s.handleAutoJoin(sessionID, out)
	}

	log.Printf("[tutor] job %s selesai: fase=%s balasan=%d karakter join=%v join_error=%q",
		jobID, out.Fase, len(out.Balasan), out.Join != nil, out.JoinError)
	return out, jobID, nil
}

// handleAutoJoin memutuskan nasib join: backend yang memegang kebenaran.
func (s *TutorService) handleAutoJoin(sessionID string, out *ChatResult) {
	s.mu.Lock()
	last := s.sessionLastCode[sessionID]
	failed := s.sessionFailedCode[sessionID]
	s.mu.Unlock()

	// Prioritaskan kode yang benar-benar disebut siswa; fallback ke extract AI
	candidate := last
	if candidate == "" {
		candidate = out.ExtractKode
	}

	if !kodeKelasValid(candidate) {
		out.JoinError = "Kode kelas harus enam digit angka."
		out.Fase = "onboarding"
		out.Balasan = "Kode kelas terdiri dari enam digit angka. Coba sebutkan lagi ya."
		out.ExtractKode = ""
		return
	}

	// Kode ini sudah gagal sebelumnya dan belum ada kode baru -> jangan retry buta,
	// minta kode lain (mengatasi extract AI yang lengket di kode lama)
	if candidate == failed {
		out.JoinError = "kelas dengan kode '" + candidate + "' tidak ditemukan"
		out.Fase = "onboarding"
		out.Balasan = fmt.Sprintf("Kode %s belum terdaftar di sistem. Sebutkan kode kelas yang lain ya.", candidate)
		out.ExtractKode = ""
		return
	}

	siswa, _, errJoin := s.siswaService.JoinSiswa(candidate, out.ExtractNama)
	if errJoin != nil {
		s.mu.Lock()
		s.sessionFailedCode[sessionID] = candidate
		s.mu.Unlock()
		out.JoinError = errJoin.Error()
		out.Fase = "onboarding"
		out.Balasan = fmt.Sprintf("Maaf, kode kelas %s tidak ditemukan. Coba sebutkan lagi kode kelas kamu ya.", candidate)
		out.ExtractKode = ""
		log.Printf("[tutor] auto-join gagal: %v", errJoin)
		return
	}

	// SUKSES: ikat session ke siswa, umumkan resmi oleh backend
	if errLink := s.siswaService.LinkSession(siswa.ID, sessionID); errLink != nil {
		log.Printf("[tutor] warning: gagal ikat session: %v", errLink)
	}
	s.mu.Lock()
	delete(s.sessionFailedCode, sessionID)
	s.mu.Unlock()

	namaKelas, _ := s.siswaService.NamaKelasByID(siswa.KelasID)
	out.Join = &JoinInfo{
		SiswaID:   siswa.ID,
		KelasID:   siswa.KelasID,
		Nama:      siswa.Nama,
		KelasNama: namaKelas,
	}
	out.Fase = "belajar"
	out.Balasan = fmt.Sprintf("%s Kamu sekarang resmi masuk kelas %s. Selamat belajar, %s!",
		out.Balasan, namaKelas, siswa.Nama)
	log.Printf("[tutor] auto-join SUKSES: siswa %d (%s) kelas %d (%s), session terikat",
		siswa.ID, siswa.Nama, siswa.KelasID, namaKelas)
}