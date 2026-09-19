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
	sessionNama       map[string]string // session -> nama siswa yang valid
}

var (
	sixDigits    = regexp.MustCompile(`\b\d{6}\b`)
	konfirmasiRe = regexp.MustCompile(`\b(ya|iya|benar|betul)\b`)
	negasiRe     = regexp.MustCompile(`\b(belum|bukan|salah|tidak)\b`)

	// Pola frasa eksplisit untuk menangkap nama
	namaPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bnama\s+aku\s+([a-zA-Z][a-zA-Z'\-]{1,20}(?:\s[a-zA-Z][a-zA-Z'\-]{1,20})?)`),
		regexp.MustCompile(`(?i)\bnamaku\s+([a-zA-Z][a-zA-Z'\-]{1,20}(?:\s[a-zA-Z][a-zA-Z'\-]{1,20})?)`),
		regexp.MustCompile(`(?i)\bnama\s+saya\s+([a-zA-Z][a-zA-Z'\-]{1,20}(?:\s[a-zA-Z][a-zA-Z'\-]{1,20})?)`),
		regexp.MustCompile(`(?i)\bpanggil\s+aku\s+([a-zA-Z][a-zA-Z'\-]{1,20}(?:\s[a-zA-Z][a-zA-Z'\-]{1,20})?)`),
	}

	stopWordsNama = map[string]bool{
		"adalah": true, "itu": true, "ini": true, "aku": true, "saya": true,
		"gua": true, "gue": true, "namanya": true, "nama": true, "dia": true,
	}
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

// sanitizeNama membersihkan nama hasil extract AI / ucapan user.
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
	if stopWordsNama[lower] {
		return ""
	}
	if konfirmasiRe.MatchString(lower) && len(strings.Fields(n)) <= 2 {
		return ""
	}
	badWords := []string{
		"siap", "belajar", "momo", "halo", "oke", "baik", "boleh", "mulai",
		"lanjut", "ayo", "silakan", "terima", "belum", "tidak", "maaf",
	}
	for _, w := range badWords {
		if strings.Contains(lower, w) {
			return ""
		}
	}
	return n
}

// extractNamaDariPesan menangkap nama dari frasa eksplisit ("nama aku Budi").
func extractNamaDariPesan(pesan string) string {
	for _, re := range namaPatterns {
		if m := re.FindStringSubmatch(pesan); len(m) > 1 {
			if nama := sanitizeNama(m[1]); nama != "" {
				return nama
			}
		}
	}
	return ""
}

// extractNamaFallback menangkap nama polos ("Budi", "aku Budi") saat fase nama
// masih kosong: pesan pendek, murni huruf, dan lolos sanitize.
func extractNamaFallback(pesan string) string {
	p := strings.TrimSpace(pesan)
	lower := strings.ToLower(p)
	prefixes := []string{
		"nama aku ", "namaku ", "nama saya ", "panggil aku ",
		"aku ", "saya ", "gua ", "gue ", "ini ",
	}
	for _, pre := range prefixes {
		if strings.HasPrefix(lower, pre) {
			p = strings.TrimSpace(p[len(pre):])
			lower = strings.ToLower(p)
			break
		}
	}
	p = strings.TrimSpace(strings.ReplaceAll(p, "adalah", " "))
	if p == "" || len(p) > 30 || strings.Count(p, " ") > 2 {
		return ""
	}
	for _, r := range p {
		if r != ' ' && r != '\'' && r != '-' && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			return ""
		}
	}
	return sanitizeNama(p)
}

// extractTopik mengambil inti topik dari pesan user ("aku mau belajar getaran" -> "getaran").
func extractTopik(pesan string) string {
	p := strings.ToLower(strings.TrimSpace(pesan))
	prefixes := []string{
		"aku mau belajar tentang", "saya mau belajar tentang", "aku ingin belajar tentang",
		"aku mau belajar", "saya mau belajar", "aku ingin belajar", "saya ingin belajar",
		"mau belajar tentang", "ingin belajar tentang", "mau belajar", "ingin belajar",
		"belajar tentang", "materi tentang", "belajar", "materi", "tentang",
	}
	for _, pre := range prefixes {
		if strings.HasPrefix(p, pre) {
			p = strings.TrimSpace(strings.TrimPrefix(p, pre))
			break
		}
	}
	p = strings.Trim(p, " .!?")
	return p
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

	// 2. KONTEKS KE AI: status onboarding supaya AI tidak nyasar
	konteks := fmt.Sprintf("Kelas: %s | Session: %s", kelasNama, sessionID)
	if alreadyJoined {
		konteks += fmt.Sprintf(" | STATUS: ONBOARDING SELESAI. Siswa sudah terdaftar di kelas %s. JANGAN minta kode kelas atau nama lagi; langsung lanjut mode belajar.", storedKelas)
	} else {
		konteks += " | INSTRUKSI KRITIS: Ikuti urutan onboarding INI PERSIS: (1) tanya kesiapan, (2) kalau siap TANYA NAMA DULU, (3) baru tanya kode kelas 6 digit, (4) konfirmasi kode, (5) tunggu konfirmasi siswa. JANGAN minta kode kelas sebelum tahu nama siswa. JANGAN tanya nama dua kali. JANGAN anggap kalimat 'siap/ya/oke/halo' sebagai nama."
		s.mu.Lock()
		storedNamaCtx := s.sessionNama[sessionID]
		s.mu.Unlock()
		if storedNamaCtx != "" {
			konteks += fmt.Sprintf(" | Nama siswa sudah diketahui: %s. Jangan tanya nama lagi; lanjut minta kode kelas.", storedNamaCtx)
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

	// 3. REKAM NAMA: frasa eksplisit dulu, lalu fallback nama polos
	namaTurn := extractNamaDariPesan(pesanSiswa)
	if namaTurn == "" && !alreadyJoined && !sixDigits.MatchString(pesanSiswa) {
		namaTurn = extractNamaFallback(pesanSiswa)
	}
	if namaTurn != "" {
		s.mu.Lock()
		s.sessionNama[sessionID] = namaTurn
		s.mu.Unlock()
		if out.ExtractNama == "" {
			out.ExtractNama = namaTurn
		}
	} else if out.ExtractNama != "" {
		s.mu.Lock()
		s.sessionNama[sessionID] = out.ExtractNama
		s.mu.Unlock()
	}

	s.mu.Lock()
	storedNama := s.sessionNama[sessionID]
	s.mu.Unlock()

	// 4. DETEKSI pola balasan AI (dengan penjaga negasi anti false-positive)
	balasanLower := strings.ToLower(out.Balasan)
	adaNegasi := strings.Contains(balasanLower, "tidak perlu") ||
		strings.Contains(balasanLower, "sudah masuk") ||
		strings.Contains(balasanLower, "sudah terdaftar") ||
		strings.Contains(balasanLower, "sudah terkonfirmasi") ||
		strings.Contains(balasanLower, "sudah benar") ||
		strings.Contains(balasanLower, "sudah tersimpan")

	mintaKodePositif := strings.Contains(balasanLower, "sebutkan kode") ||
		strings.Contains(balasanLower, "sebutkan kembali kode") ||
		strings.Contains(balasanLower, "bisa sebutkan lagi") ||
		strings.Contains(balasanLower, "sebutkan lagi kode") ||
		strings.Contains(balasanLower, "belum terkonfirmasi") ||
		strings.Contains(balasanLower, "mohon sebutkan kode") ||
		strings.Contains(balasanLower, "kurang mendengar kode") ||
		strings.Contains(balasanLower, "tidak terdengar jelas") ||
		(strings.Contains(balasanLower, "kode kelas") && strings.Contains(balasanLower, "sebutkan lagi"))
	aiMintaKode := mintaKodePositif && !adaNegasi

	mintaNamaPositif := strings.Contains(balasanLower, "siapa nama") ||
		strings.Contains(balasanLower, "nama kamu") ||
		strings.Contains(balasanLower, "sebutkan nama") ||
		strings.Contains(balasanLower, "boleh tahu nama")
	aiMintaNama := mintaNamaPositif && !adaNegasi

	pesanLower := strings.ToLower(pesanSiswa)
	isKonfirmasi := konfirmasiRe.MatchString(pesanLower) && !negasiRe.MatchString(pesanLower)

	if !alreadyJoined {
		switch {
		// 4a. FIX BUG 1: user baru sebut nama di giliran ini tapi AI malah
		//     komplain kode -> ganti jadi ucapan terima kasih + tanya kode bersih
		case namaTurn != "" && aiMintaKode:
			out.Balasan = fmt.Sprintf("Terima kasih, %s. Sekarang sebutkan kode kelas kamu, enam digit angka.", namaTurn)
			out.Fase = "onboarding"
			log.Printf("[tutor] override: ACK nama baru (%s) + tanya kode bersih (session %s)", namaTurn, sessionID)

		// 4b. AI tanya nama padahal nama sudah diketahui -> potong, lanjut minta kode
		case aiMintaNama && storedNama != "":
			out.Balasan = fmt.Sprintf("Terima kasih, %s. Sekarang sebutkan kode kelas kamu, enam digit angka.", storedNama)
			out.Fase = "onboarding"
			log.Printf("[tutor] override: AI tanya nama ulang padahal sudah tahu (%s) (session %s)", storedNama, sessionID)

		// 4c. AI minta kode dengan frasa "ulangi/tidak jelas" padahal user belum
		//     pernah sebut kode -> ganti jadi pertanyaan kode yang bersih
		case aiMintaKode && storedNama != "" && lastCode == "":
			out.Balasan = fmt.Sprintf("%s, sekarang sebutkan kode kelas kamu, enam digit angka.", storedNama)
			out.Fase = "onboarding"
			log.Printf("[tutor] override: pertanyaan kode dibersihkan (user belum pernah sebut kode) (session %s)", sessionID)

		// 4d. AI minta kode padahal nama benar-benar belum ada -> paksa tanya nama dulu
		case aiMintaKode && storedNama == "":
			out.Balasan = "Sebelum kita lanjut ke kode kelas, boleh tahu dulu nama kamu? Sebutkan nama kamu ya."
			out.Fase = "onboarding"
			out.ExtractKode = ""
			log.Printf("[tutor] override: AI minta kode padahal nama belum ada (session %s)", sessionID)

		// 4e. Trigger auto-join: user konfirmasi + ada kode terekam, ATAU extract AI lengkap
		case (isKonfirmasi && lastCode != "") || (out.ExtractNama != "" && out.ExtractKode != ""):
			s.handleAutoJoin(sessionID, out)
		}
	} else {
		// 4f. FIX BUG 2: sudah join tapi AI masih minta nama/kode -> jangan ulangi
		//     kalimat yang sama; kalau user menyebut topik, jembatani ke topik itu
		if aiMintaKode || aiMintaNama {
			nama := storedNama
			if nama == "" {
				nama = "teman"
			}
			if topik := extractTopik(pesanSiswa); len(topik) > 2 {
				out.Balasan = fmt.Sprintf("Dicatat, %s! Kita akan belajar %s bersama-sama. Apa yang paling ingin kamu tahu dulu tentang itu?", nama, topik)
			} else {
				out.Balasan = fmt.Sprintf("Kamu sudah masuk kelas %s, tidak perlu kode atau nama lagi. Hari ini kamu mau belajar apa?", storedKelas)
			}
			out.Fase = "belajar"
			log.Printf("[tutor] override: AI nyasar pasca-join, dijembatani ke topik (session %s)", sessionID)
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