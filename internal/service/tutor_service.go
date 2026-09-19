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

const paddingMarker = "Siswa menjawab: "

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
	sessionLastCode   map[string]string // kode 6 digit terakhir yang disebut siswa
	sessionFailedCode map[string]string // kode yang gagal join (jangan di-retry buta)
	sessionJoined     map[string]bool   // session yang sudah sukses join
}

var (
	sixDigits    = regexp.MustCompile(`\b\d{6}\b`)
	konfirmasiRe = regexp.MustCompile(`\b(ya|iya|benar|betul)\b`)
	negasiRe     = regexp.MustCompile(`\b(belum|bukan|salah|tidak)\b`)
)

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
		sessionJoined:     make(map[string]bool),
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

// bersihPadding menghapus artefak padding backend yang kadang di-echo AI
// (misal balasan: "Terima kasih, Siswa menjawab: ya benar." -> "Terima kasih, ya benar.")
func bersihPadding(s string) string {
	return strings.ReplaceAll(s, paddingMarker, "")
}

// ProcessTutor = SATU API percakapan + AUTO-JOIN deterministik.
// kodeHint = kode 6 digit terakhir yang direkam FE dari suara siswa (opsional).
func (s *TutorService) ProcessTutor(sessionID string, kelasNama string, pesanSiswa string, kodeHint string) (*ChatResult, string, error) {
	jobID := job.GenerateID("tutor")

	s.jobRegistry.Register(jobID, job.JobTypeTutor, 0, "", sessionID)

	// Rekam kode 6 digit yang disebut siswa di giliran ini
	if m := sixDigits.FindString(pesanSiswa); m != "" {
		s.mu.Lock()
		s.sessionLastCode[sessionID] = m
		s.mu.Unlock()
	}

	konteks := fmt.Sprintf("Kelas: %s | Session: %s", kelasNama, sessionID)

	// Padding pesan pendek (kalau AI Service masih min_length 10)
	teksKirim := pesanSiswa
	if len(pesanSiswa) < 10 {
		teksKirim = paddingMarker + pesanSiswa
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
		Balasan:     bersihPadding(res.Balasan),
		Fase:        res.Fase,
		ExtractNama: bersihPadding(res.ExtractNama),
		ExtractKode: res.ExtractKode,
	}

	// Keputusan join: BACKEND yang pegang, bukan AI
	s.putuskanJoin(sessionID, pesanSiswa, kodeHint, out)

	log.Printf("[tutor] job %s selesai: fase=%s balasan=%d karakter join=%v join_error=%q",
		jobID, out.Fase, len(out.Balasan), out.Join != nil, out.JoinError)
	return out, jobID, nil
}

// putuskanJoin menjalankan auto-join dengan 2 trigger:
//  1. DETERMINISTIK: FE kirim kode_hint + suara siswa adalah konfirmasi ("ya/benar/betul")
//  2. FALLBACK: AI mengirim extract lengkap (nama + kode)
func (s *TutorService) putuskanJoin(sessionID string, pesanSiswa string, kodeHint string, out *ChatResult) {
	s.mu.Lock()
	last := s.sessionLastCode[sessionID]
	failed := s.sessionFailedCode[sessionID]
	joined := s.sessionJoined[sessionID]
	s.mu.Unlock()

	if joined {
		return
	}

	pesanLower := strings.ToLower(pesanSiswa)
	konfirmasi := konfirmasiRe.MatchString(pesanLower) && !negasiRe.MatchString(pesanLower)
	triggerHint := kodeKelasValid(kodeHint) && konfirmasi
	triggerExtract := out.ExtractNama != "" && out.ExtractKode != ""

	if !triggerHint && !triggerExtract {
		return
	}

	// Prioritas kandidat: hint FE > kode terakhir terekam > extract AI
	candidate := kodeHint
	if !kodeKelasValid(candidate) {
		candidate = last
	}
	if !kodeKelasValid(candidate) {
		candidate = out.ExtractKode
	}
	if !kodeKelasValid(candidate) {
		return
	}

	// Kode ini sudah gagal sebelumnya dan belum ada kode baru -> minta kode lain
	if candidate == failed {
		out.JoinError = "kelas dengan kode '" + candidate + "' tidak ditemukan"
		out.Fase = "onboarding"
		out.Balasan = fmt.Sprintf("Kode %s belum terdaftar di sistem. Sebutkan kode kelas yang lain ya.", candidate)
		out.ExtractKode = ""
		return
	}

	namaJoin := out.ExtractNama
	if namaJoin == "" {
		namaJoin = "Siswa"
	}

	siswa, _, errJoin := s.siswaService.JoinSiswa(candidate, namaJoin)
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

	if errLink := s.siswaService.LinkSession(siswa.ID, sessionID); errLink != nil {
		log.Printf("[tutor] warning: gagal ikat session: %v", errLink)
	}
	s.mu.Lock()
	s.sessionJoined[sessionID] = true
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