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

// JoinInfo adalah info auto-join onboarding (token = ID card setelah kode valid).
type JoinInfo struct {
	SiswaID   uint   `json:"siswa_id"`
	KelasID   uint   `json:"kelas_id"`
	Nama      string `json:"nama"`
	KelasNama string `json:"kelas_nama"`
	Token     string `json:"token"`
}

// ChatResult adalah response lengkap percakapan.
type ChatResult struct {
	JobID       string    `json:"job_id"`
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
	sessionLastCode   map[string]string // session -> kode 6 digit terakhir yang disebut siswa
	sessionFailedCode map[string]string // session -> kode yang gagal join
	sessionJoined     map[string]bool   // session yang sudah sukses join
	sessionKelasNama  map[string]string // session -> nama kelas setelah join
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
		sessionKelasNama:  make(map[string]string),
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

// sanitizeNama membersihkan nama hasil extract AI.
// Return "" kalau nama tidak masuk akal (padding, kata konfirmasi, kata kesiapan).
func sanitizeNama(nama string) string {
	n := strings.TrimSpace(nama)
	if idx := strings.Index(n, "menjawab:"); idx >= 0 {
		n = n[idx+len("menjawab:"):]
	}
	n = strings.TrimSpace(n)
	if n == "" {
		return ""
	}
	lower := strings.ToLower(n)
	// tolak kalau isinya cuma konfirmasi ("ya benar")
	if konfirmasiRe.MatchString(lower) && len(strings.Fields(n)) <= 2 {
		return ""
	}
	// tolak kalau sebenarnya kalimat kesiapan/sapaan yang ditangkap sebagai nama
	if strings.Contains(lower, "siap") || strings.Contains(lower, "belajar") ||
		strings.Contains(lower, "momo") || strings.Contains(lower, "halo") {
		return ""
	}
	return n
}

// bersihPadding menghapus artefak padding dari balasan AI.
func bersihPadding(s string) string {
	return strings.ReplaceAll(s, paddingMarker, "")
}

// ProcessTutor = SATU API percakapan + AUTO-JOIN onboarding.
func (s *TutorService) ProcessTutor(sessionID string, kelasNama string, pesanSiswa string) (*ChatResult, error) {
	jobID := job.GenerateID("tutor")
	s.jobRegistry.Register(jobID, job.JobTypeTutor, 0, "", sessionID)

	// 1. REKAM kode 6 digit yang disebut siswa di giliran ini
	if m := sixDigits.FindString(pesanSiswa); m != "" {
		s.mu.Lock()
		s.sessionLastCode[sessionID] = m
		s.mu.Unlock()
	}

	s.mu.Lock()
	alreadyJoined := s.sessionJoined[sessionID]
	storedKelas := s.sessionKelasNama[sessionID]
	lastCode := s.sessionLastCode[sessionID]
	s.mu.Unlock()

	// 2. KONTEKS KE AI: beri tahu status onboarding supaya AI tidak minta kode lagi
	konteks := fmt.Sprintf("Kelas: %s | Session: %s", kelasNama, sessionID)
	if alreadyJoined {
		konteks += fmt.Sprintf(" | STATUS: ONBOARDING SELESAI. Siswa sudah terdaftar di kelas %s. JANGAN minta kode kelas lagi; langsung lanjut mode belajar.", storedKelas)
	}

	teksKirim := pesanSiswa
	if len(pesanSiswa) < 10 {
		teksKirim = paddingMarker + pesanSiswa
	}

	err := s.aiClient.SubmitTutor(jobID, s.callbackURL, teksKirim, konteks, sessionID)
	if err != nil {
		log.Printf("[tutor] gagal kirim job %s: %v", jobID, err)
		return nil, fmt.Errorf("gagal menghubungi AI Service: %w", err)
	}

	log.Printf("[tutor] job %s (session %s) dikirim, menunggu reasoning AI...", jobID, sessionID)

	finished := s.jobRegistry.WaitSync(jobID, 90*time.Second)
	if finished == nil {
		return nil, fmt.Errorf("timeout menunggu balasan tutor (90 detik)")
	}
	if finished.Status == "failed" {
		return nil, fmt.Errorf("AI tutor gagal memproses: %s", finished.Error)
	}

	res, err := job.ParseTutorFull(finished.Hasil)
	if err != nil {
		return nil, fmt.Errorf("gagal parse balasan tutor: %w", err)
	}

	out := &ChatResult{
		JobID:       jobID,
		Balasan:     bersihPadding(res.Balasan),
		Fase:        res.Fase,
		ExtractNama: sanitizeNama(res.ExtractNama),
		ExtractKode: res.ExtractKode,
	}

	// 3. TRIGGER AUTO-JOIN hanya jika user konfirmasi DAN ada kode terekam
	pesanLower := strings.ToLower(pesanSiswa)
	isKonfirmasi := konfirmasiRe.MatchString(pesanLower) && !negasiRe.MatchString(pesanLower)

	if !alreadyJoined {
		if (isKonfirmasi && lastCode != "") || (out.ExtractNama != "" && out.ExtractKode != "") {
			s.handleAutoJoin(sessionID, out)
		}
	} else {
		// 4. PENGAMAN: sudah join tapi AI masih minta kode -> ganti balasannya
		balasanLower := strings.ToLower(out.Balasan)
		mintaKode := strings.Contains(balasanLower, "kode kelas") &&
			(strings.Contains(balasanLower, "sebutkan") || strings.Contains(balasanLower, "belum benar") ||
				strings.Contains(balasanLower, "belum terkonfirmasi") || strings.Contains(balasanLower, "sebutkan lagi"))
		if mintaKode {
			out.Balasan = fmt.Sprintf("Kamu sudah masuk kelas %s, tidak perlu kode lagi. Hari ini kamu mau belajar apa?", storedKelas)
			out.Fase = "belajar"
			log.Printf("[tutor] override: AI minta kode padahal sudah join (session %s)", sessionID)
		}
	}

	log.Printf("[tutor] job %s selesai: fase=%s balasan=%d karakter join=%v join_error=%q",
		jobID, out.Fase, len(out.Balasan), out.Join != nil, out.JoinError)
	return out, nil
}

// handleAutoJoin memutuskan nasib join: backend yang memegang kebenaran.
func (s *TutorService) handleAutoJoin(sessionID string, out *ChatResult) {
	s.mu.Lock()
	last := s.sessionLastCode[sessionID]
	failed := s.sessionFailedCode[sessionID]
	joined := s.sessionJoined[sessionID]
	s.mu.Unlock()

	if joined {
		return
	}

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

	siswa, token, errJoin := s.siswaService.JoinSiswa(candidate, namaJoin)
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

	// SUKSES: simpan status + GANTI balasan dengan pesan sukses yang bersih
	namaKelas, _ := s.siswaService.NamaKelasByID(siswa.KelasID)
	s.mu.Lock()
	s.sessionJoined[sessionID] = true
	s.sessionKelasNama[sessionID] = namaKelas
	delete(s.sessionFailedCode, sessionID)
	s.mu.Unlock()

	out.Join = &JoinInfo{
		SiswaID:   siswa.ID,
		KelasID:   siswa.KelasID,
		Nama:      siswa.Nama,
		KelasNama: namaKelas,
		Token:     token,
	}
	out.Fase = "belajar"
	out.Balasan = fmt.Sprintf("Sempurna! Kamu sekarang resmi masuk kelas %s. Selamat belajar, %s! Hari ini kamu mau belajar apa?",
		namaKelas, siswa.Nama)
	log.Printf("[tutor] auto-join SUKSES: siswa %d (%s) kelas %d (%s)",
		siswa.ID, siswa.Nama, siswa.KelasID, namaKelas)
}