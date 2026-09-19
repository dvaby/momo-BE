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
	sessionFailedCode map[string]string // session -> kode yang gagal join (jangan di-retry buta)
	sessionJoined     map[string]bool   // session yang sudah sukses join
	sessionKelasNama  map[string]string // session -> nama kelas setelah join
	sessionNama       map[string]string // session -> nama siswa yang valid
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
		sessionNama:       make(map[string]string),
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
// Return "" kalau nama tidak masuk akal (padding, kata konfirmasi, kata kesiapan/sapaan).
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
		strings.Contains(lower, "momo") || strings.Contains(lower, "halo") ||
		strings.Contains(lower, "oke") || strings.Contains(lower, "baik") {
		return ""
	}
	return n
}

// bersihPadding menghapus artefak padding dari balasan AI.
func bersihPadding(s string) string {
	return strings.ReplaceAll(s, paddingMarker, "")
}

// ProcessTutor = SATU API percakapan + AUTO-JOIN onboarding.
// Backend pemegang kebenaran: balasan final ditentukan hasil join, bukan klaim AI.
func (s *TutorService) ProcessTutor(sessionID string, kelasNama string, pesanSiswa string) (*ChatResult, error) {
	jobID := job.GenerateID("tutor")
	s.jobRegistry.Register(jobID, job.JobTypeTutor, 0, "", sessionID)

	// 1. REKAM kode 6 digit yang disebut siswa di giliran ini (sumber kebenaran utama)
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

	// 2. KONTEKS KE AI: beri tahu status onboarding supaya AI tidak nyasar
	konteks := fmt.Sprintf("Kelas: %s | Session: %s", kelasNama, sessionID)
	if alreadyJoined {
		konteks += fmt.Sprintf(" | STATUS: ONBOARDING SELESAI. Siswa sudah terdaftar di kelas %s. JANGAN minta kode kelas atau nama lagi; langsung lanjut mode belajar.", storedKelas)
	} else {
		konteks += " | INSTRUKSI KRITIS: Ikuti urutan onboarding INI PERSIS: (1) tanya kesiapan, (2) kalau siap TANYA NAMA DULU, (3) baru tanya kode kelas 6 digit, (4) konfirmasi kode, (5) tunggu konfirmasi siswa. JANGAN minta kode kelas sebelum tahu nama siswa. JANGAN anggap kalimat 'siap/ya/oke/halo' sebagai nama."
		s.mu.Lock()
		storedNama := s.sessionNama[sessionID]
		s.mu.Unlock()
		if storedNama != "" {
			konteks += fmt.Sprintf(" | Nama siswa sudah diketahui: %s. Jangan tanya nama lagi; lanjut minta kode kelas.", storedNama)
		}
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

	// 3. SIMPAN nama valid ke memori session
	if out.ExtractNama != "" {
		s.mu.Lock()
		s.sessionNama[sessionID] = out.ExtractNama
		s.mu.Unlock()
	}

	s.mu.Lock()
	storedNama := s.sessionNama[sessionID]
	s.mu.Unlock()

	// 4. DETEKSI pola balasan AI
	balasanLower := strings.ToLower(out.Balasan)
	aiMintaKode := strings.Contains(balasanLower, "kode kelas") &&
		(strings.Contains(balasanLower, "sebutkan") || strings.Contains(balasanLower, "bisa sebutkan lagi") ||
			strings.Contains(balasanLower, "belum terkonfirmasi") || strings.Contains(balasanLower, "belum benar"))
	aiMintaNama := strings.Contains(balasanLower, "siapa nama") || strings.Contains(balasanLower, "nama kamu")

	pesanLower := strings.ToLower(pesanSiswa)
	isKonfirmasi := konfirmasiRe.MatchString(pesanLower) && !negasiRe.MatchString(pesanLower)

	if !alreadyJoined {
		// 4a. AI minta kode padahal nama belum diketahui -> paksa tanya nama dulu
		if aiMintaKode && storedNama == "" {
			out.Balasan = "Sebelum kita lanjut ke kode kelas, boleh tahu dulu nama kamu? Sebutkan nama kamu ya."
			out.Fase = "onboarding"
			out.ExtractKode = ""
			log.Printf("[tutor] override: AI minta kode padahal nama belum ada (session %s)", sessionID)
		} else if (isKonfirmasi && lastCode != "") || (out.ExtractNama != "" && out.ExtractKode != "") {
			// 4b. Trigger auto-join: user konfirmasi + ada kode terekam, ATAU extract AI lengkap
			s.handleAutoJoin(sessionID, out)
		}
	} else {
		// 4c. Sudah join: pastikan AI tidak minta kode/nama lagi
		if aiMintaKode || aiMintaNama {
			out.Balasan = fmt.Sprintf("Kamu sudah masuk kelas %s, tidak perlu kode atau nama lagi. Hari ini kamu mau belajar apa?", storedKelas)
			out.Fase = "belajar"
			log.Printf("[tutor] override: AI minta kode/nama padahal sudah join (session %s)", sessionID)
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
	storedNama := s.sessionNama[sessionID]
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

	// Cooldown: kode ini sudah gagal sebelumnya -> jangan retry buta, minta kode lain
	if candidate == failed {
		out.JoinError = "kelas dengan kode '" + candidate + "' tidak ditemukan"
		out.Fase = "onboarding"
		out.Balasan = fmt.Sprintf("Kode %s belum terdaftar di sistem. Sebutkan kode kelas yang lain ya.", candidate)
		out.ExtractKode = ""
		return
	}

	namaJoin := out.ExtractNama
	if namaJoin == "" {
		namaJoin = storedNama
	}
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
	s.sessionNama[sessionID] = siswa.Nama
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