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
	aiClient          *aiclient.Client
	jobRegistry       *job.Registry
	callbackURL       string
	siswaService      *SiswaService
	mu                sync.Mutex
	sessionLastCode   map[string]string // session -> kode 6 digit terakhir yang disebut siswa
	sessionFailedCode map[string]string // session -> kode yang gagal join
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

// sanitizeNama membersihkan artefak padding dan kata konfirmasi dari nama.
func sanitizeNama(nama string) string {
	n := strings.TrimSpace(nama)
	if idx := strings.Index(n, "menjawab:"); idx >= 0 {
		n = n[idx+len("menjawab:"):]
	}
	return strings.TrimSpace(n)
}

// bersihPadding menghapus artefak padding dari balasan AI.
func bersihPadding(s string) string {
	return strings.ReplaceAll(s, paddingMarker, "")
}

// ProcessTutor mengembalikan 2 value: (*ChatResult, error) untuk mencegah mismatch di handler.
func (s *TutorService) ProcessTutor(sessionID string, kelasNama string, pesanSiswa string) (*ChatResult, error) {
	jobID := job.GenerateID("tutor")
	s.jobRegistry.Register(jobID, job.JobTypeTutor, 0, "", sessionID)

	// 1. REKAM kode 6 digit yang disebut siswa di giliran ini (sumber kebenaran utama)
	if m := sixDigits.FindString(pesanSiswa); m != "" {
		s.mu.Lock()
		s.sessionLastCode[sessionID] = m
		s.mu.Unlock()
	}

	konteks := fmt.Sprintf("Kelas: %s | Session: %s", kelasNama, sessionID)
	teksKirim := pesanSiswa
	if len(pesanSiswa) < 10 {
		teksKirim = "Siswa menjawab: " + pesanSiswa
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
		JobID:       jobID, // Masukkan job_id ke dalam struct result
		Balasan:     bersihPadding(res.Balasan),
		Fase:        res.Fase,
		ExtractNama: sanitizeNama(res.ExtractNama),
		ExtractKode: res.ExtractKode,
	}

	// 2. CEK APAKAH USER SEDANG KONFIRMASI ("ya", "benar", "betul")
	pesanLower := strings.ToLower(pesanSiswa)
	isKonfirmasi := konfirmasiRe.MatchString(pesanLower) && !negasiRe.MatchString(pesanLower)

	s.mu.Lock()
	lastCode := s.sessionLastCode[sessionID]
	joined := s.sessionJoined[sessionID]
	s.mu.Unlock()

	// 3. TRIGGER AUTO-JOIN HANYA JIKA:
	//    - User konfirmasi ("ya benar") DAN ada kode yang direkam sebelumnya
	//    - ATAU AI langsung kirim extract lengkap (fallback)
	shouldTryJoin := false
	if !joined {
		if isKonfirmasi && lastCode != "" {
			shouldTryJoin = true
		} else if out.ExtractNama != "" && out.ExtractKode != "" {
			shouldTryJoin = true
		}
	}

	if shouldTryJoin {
		s.handleAutoJoin(sessionID, out)
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

	// Cooldown: Kode ini sudah gagal sebelumnya -> jangan retry buta
	if candidate == failed {
		out.JoinError = "kelas dengan kode '" + candidate + "' tidak ditemukan"
		out.Fase = "onboarding"
		out.Balasan = fmt.Sprintf("Kode %s belum terdaftar di sistem. Sebutkan kode kelas yang lain ya.", candidate)
		out.ExtractKode = ""
		return
	}

	// Sanitize nama: reject kalau isinya cuma kata konfirmasi ("ya", "benar")
	namaJoin := sanitizeNama(out.ExtractNama)
	namaLower := strings.ToLower(namaJoin)
	konfirmasiOnly := konfirmasiRe.MatchString(namaLower) && len(strings.Fields(namaJoin)) <= 2

	if namaJoin == "" || konfirmasiOnly {
		namaJoin = "Siswa" // fallback ke nama generik
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

	// SUKSES: umumkan resmi oleh backend
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
		Token:     token,
	}
	out.Fase = "belajar"
	out.Balasan = fmt.Sprintf("%s Kamu sekarang resmi masuk kelas %s. Selamat belajar, %s!",
		out.Balasan, namaKelas, siswa.Nama)
	log.Printf("[tutor] auto-join SUKSES: siswa %d (%s) kelas %d (%s)",
		siswa.ID, siswa.Nama, siswa.KelasID, namaKelas)
}