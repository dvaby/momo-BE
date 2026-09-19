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

type JoinInfo struct {
	SiswaID   uint   `json:"siswa_id"`
	KelasID   uint   `json:"kelas_id"`
	Nama      string `json:"nama"`
	KelasNama string `json:"kelas_nama"`
	Token     string `json:"token"`
}

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
	sessionLastCode   map[string]string
	sessionFailedCode map[string]string
	sessionJoined     map[string]bool
	sessionKelasNama  map[string]string
	sessionNama       map[string]string
	sessionKelasID    map[string]uint
	sessionKonten     map[string]KontenKelas
}

var (
	sixDigits    = regexp.MustCompile(`\b\d{6}\b`)
	konfirmasiRe = regexp.MustCompile(`\b(ya|iya|benar|betul)\b`)
	negasiRe     = regexp.MustCompile(`\b(belum|bukan|salah|tidak)\b`)

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
		sessionKelasID:    make(map[string]uint),
		sessionKonten:     make(map[string]KontenKelas),
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
		"sedang", "mendengarkan", "emang", "tadi", "salah", "sudah", "benar",
	}
	for _, w := range badWords {
		if strings.Contains(lower, w) {
			return ""
		}
	}
	return n
}

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

func deteksiNiatUser(pesan string) (string, string) {
	p := strings.ToLower(strings.TrimSpace(pesan))

	var niat string
	if strings.Contains(p, "materi") || strings.Contains(p, "baca") || strings.Contains(p, "belajar teori") {
		niat = "materi"
	} else if strings.Contains(p, "soal") || strings.Contains(p, "latihan") || strings.Contains(p, "kerjain") || strings.Contains(p, "ngerjain") || strings.Contains(p, "ujian") || strings.Contains(p, "quis") || strings.Contains(p, "kuis") {
		niat = "soal"
	}

	topik := ""
	prefixes := []string{
		"aku mau belajar tentang", "saya mau belajar tentang", "aku ingin belajar tentang",
		"aku mau belajar", "saya mau belajar", "aku ingin belajar", "saya ingin belajar",
		"mau belajar tentang", "ingin belajar tentang", "mau belajar", "ingin belajar",
		"belajar tentang", "materi tentang", "soal tentang", "tentang", "materi", "soal",
		"baca tentang", "baca materi", "kerjain soal", "ngerjain soal",
	}
	for _, pre := range prefixes {
		if strings.HasPrefix(p, pre) {
			topik = strings.TrimSpace(strings.TrimPrefix(p, pre))
			break
		}
	}
	topik = strings.Trim(topik, " .!?")
	return niat, topik
}

func bersihPadding(s string) string {
	return strings.ReplaceAll(s, paddingMarker, "")
}

// cekKataOnboarding mendeteksi apakah balasan AI masih mengandung kata-kata onboarding.
// Dipakai untuk override agresif pasca-join.
func cekKataOnboarding(balasanLower string) bool {
	kataOnboarding := []string{
		"kode kelas", "kode belum", "belum benar", "belum terkonfirmasi",
		"sebutkan kode", "sebutkan kembali", "sebutkan lagi", "sebutkan nama",
		"enam digit", "siapa nama", "nama kamu", "boleh tahu nama",
		"belum terdaftar", "kode tidak", "kodenya", "ulangi kode",
	}
	for _, kata := range kataOnboarding {
		if strings.Contains(balasanLower, kata) {
			return true
		}
	}
	return false
}

func (s *TutorService) ProcessTutor(sessionID string, kelasNama string, pesanSiswa string) (*ChatResult, error) {
	jobID := job.GenerateID("tutor")
	s.jobRegistry.Register(jobID, job.JobTypeTutor, 0, "", sessionID)

	// 1. REKAM kode 6 digit HANYA dari ucapan siswa (sumber kebenaran utama)
	if m := sixDigits.FindString(pesanSiswa); m != "" {
		s.mu.Lock()
		s.sessionLastCode[sessionID] = m
		s.mu.Unlock()
	}

	s.mu.Lock()
	alreadyJoined := s.sessionJoined[sessionID]
	storedKelas := s.sessionKelasNama[sessionID]
	lastCode := s.sessionLastCode[sessionID]
	storedKelasID := s.sessionKelasID[sessionID]
	storedKonten := s.sessionKonten[sessionID]
	s.mu.Unlock()

	konteks := fmt.Sprintf("Kelas: %s | Session: %s", kelasNama, sessionID)
	if alreadyJoined {
		konteks += fmt.Sprintf(" | STATUS: ONBOARDING SELESAI. Siswa sudah terdaftar di kelas %s.", storedKelas)
		konteks += fmt.Sprintf(" | KONTEN KELAS: %d materi, %d soal tersedia.", storedKonten.JumlahMateri, storedKonten.JumlahSoal)
		konteks += " JANGAN minta kode kelas atau nama lagi; langsung lanjut mode belajar."
	} else {
		konteks += " | INSTRUKSI KRITIS: Ikuti urutan onboarding INI PERSIS: (1) tanya kesiapan, (2) kalau siap TANYA NAMA DULU, (3) baru tanya kode kelas 6 digit, (4) konfirmasi kode, (5) tunggu konfirmasi siswa. JANGAN minta kode kelas sebelum tahu nama siswa. JANGAN tanya nama dua kali."
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

	namaTurn := extractNamaDariPesan(pesanSiswa)
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

	balasanLower := strings.ToLower(out.Balasan)

	// Deteksi AI minta kode/nama
	mintaKodePositif := strings.Contains(balasanLower, "sebutkan kode") ||
		strings.Contains(balasanLower, "sebutkan kembali kode") ||
		strings.Contains(balasanLower, "bisa sebutkan lagi") ||
		strings.Contains(balasanLower, "sebutkan lagi kode") ||
		strings.Contains(balasanLower, "belum terkonfirmasi") ||
		strings.Contains(balasanLower, "mohon sebutkan kode") ||
		strings.Contains(balasanLower, "kurang mendengar kode") ||
		strings.Contains(balasanLower, "tidak terdengar jelas") ||
		(strings.Contains(balasanLower, "kode kelas") && strings.Contains(balasanLower, "sebutkan lagi"))

	// Jangan anggap sebagai "minta kode" kalau AI sedang memberi feedback jumlah digit
	adaFeedbackDigit := strings.Contains(balasanLower, "lima digit") ||
		strings.Contains(balasanLower, "tujuh digit") ||
		strings.Contains(balasanLower, "kurang dari enam") ||
		strings.Contains(balasanLower, "lebih dari enam") ||
		strings.Contains(balasanLower, "harus terdiri dari enam")

	aiMintaKode := mintaKodePositif && !adaFeedbackDigit

	mintaNamaPositif := strings.Contains(balasanLower, "siapa nama") ||
		strings.Contains(balasanLower, "nama kamu") ||
		strings.Contains(balasanLower, "sebutkan nama") ||
		strings.Contains(balasanLower, "boleh tahu nama")
	aiMintaNama := mintaNamaPositif

	pesanLower := strings.ToLower(pesanSiswa)
	isKonfirmasi := konfirmasiRe.MatchString(pesanLower) && !negasiRe.MatchString(pesanLower)

	if !alreadyJoined {
		switch {
		case namaTurn != "" && aiMintaKode:
			out.Balasan = fmt.Sprintf("Terima kasih, %s. Sekarang sebutkan kode kelas kamu, enam digit angka.", namaTurn)
			out.Fase = "onboarding"
		case aiMintaNama && storedNama != "":
			out.Balasan = fmt.Sprintf("Terima kasih, %s. Sekarang sebutkan kode kelas kamu, enam digit angka.", storedNama)
			out.Fase = "onboarding"
		case aiMintaKode && storedNama != "" && lastCode == "":
			out.Balasan = fmt.Sprintf("%s, sekarang sebutkan kode kelas kamu, enam digit angka.", storedNama)
			out.Fase = "onboarding"
		case aiMintaKode && storedNama == "":
			out.Balasan = "Sebelum kita lanjut ke kode kelas, boleh tahu dulu nama kamu? Sebutkan nama kamu ya."
			out.Fase = "onboarding"
			out.ExtractKode = ""

		// TRIGGER AUTO-JOIN: HANYA jika user konfirmasi DAN ada kode 6 digit dari ucapan user.
		// TIDAK ADA lagi trigger berdasarkan ExtractKode AI (mencegah halusinasi "111111").
		case isKonfirmasi && lastCode != "":
			s.handleAutoJoin(sessionID, out)
		}

		// SAFEGUARD ANTI-HALUSINASI FASE:
		// Jika AI nekat set fase="belajar" padahal backend belum join, paksa kembali ke onboarding.
		if out.Fase == "belajar" && out.Join == nil {
			out.Fase = "onboarding"
			if storedNama != "" {
				out.Balasan = fmt.Sprintf("Wah, sepertinya aku salah dengar. %s, bisa tolong sebutkan ulang kode kelas kamu (enam digit angka)?", storedNama)
			} else {
				out.Balasan = "Wah, sepertinya aku salah dengar. Bisa tolong sebutkan ulang kode kelas kamu (enam digit angka)?"
			}
			log.Printf("[tutor] safeguard: AI set fase=belajar tapi user belum join. Dipaksa ke onboarding.")
		}
	} else {
		// SUDAH JOIN: PAKSA FASE BELAJAR + OVERRIDE AGRESIF
		out.Fase = "belajar"

		niat, topik := deteksiNiatUser(pesanSiswa)
		_ = storedKelasID

		nama := storedNama
		if nama == "" {
			nama = "Siswa"
		}

		// OVERRIDE AGRESIF: Kalau balasan AI masih mengandung kata onboarding, GANTI total.
		// Ini mencegah AI terus bilang "kodenya belum benar" setelah join sukses.
		if cekKataOnboarding(balasanLower) {
			if niat == "materi" && storedKonten.JumlahMateri == 0 && storedKonten.JumlahSoal > 0 {
				out.Balasan = fmt.Sprintf("Hmm, di kelas %s ini belum ada materi bacaan, %s. Yang tersedia baru %d soal latihan. Mau kita mulai ngerjain soal dulu?",
					storedKelas, nama, storedKonten.JumlahSoal)
			} else if niat == "soal" && storedKonten.JumlahSoal == 0 && storedKonten.JumlahMateri > 0 {
				out.Balasan = fmt.Sprintf("Hmm, di kelas %s ini belum ada soal latihan, %s. Yang tersedia baru %d materi bacaan. Mau kita mulai baca materi dulu?",
					storedKelas, nama, storedKonten.JumlahMateri)
			} else if len(topik) > 2 {
				out.Balasan = fmt.Sprintf("Oke %s! Ayo kita bahas %s. Apa yang ingin kamu ketahui dulu tentang itu?", nama, topik)
			} else {
				out.Balasan = fmt.Sprintf("Kamu sudah masuk kelas %s, %s. Tidak perlu kode atau nama lagi. Hari ini kamu mau belajar apa? Di kelas ini ada %d soal latihan yang bisa kamu kerjakan.",
					storedKelas, nama, storedKonten.JumlahSoal)
			}
			log.Printf("[tutor] override agresif: AI menyebut kata onboarding pasca-join (session %s)", sessionID)
		} else if niat != "" {
			// AI sudah benar, tapi cek ketersediaan konten
			if niat == "materi" && storedKonten.JumlahMateri == 0 && storedKonten.JumlahSoal > 0 {
				out.Balasan = fmt.Sprintf("Hmm, di kelas %s ini belum ada materi bacaan, %s. Yang tersedia baru %d soal latihan. Mau kita mulai ngerjain soal dulu?",
					storedKelas, nama, storedKonten.JumlahSoal)
			} else if niat == "soal" && storedKonten.JumlahSoal == 0 && storedKonten.JumlahMateri > 0 {
				out.Balasan = fmt.Sprintf("Hmm, di kelas %s ini belum ada soal latihan, %s. Yang tersedia baru %d materi bacaan. Mau kita mulai baca materi dulu?",
					storedKelas, nama, storedKonten.JumlahMateri)
			} else if storedKonten.JumlahMateri == 0 && storedKonten.JumlahSoal == 0 {
				out.Balasan = fmt.Sprintf("Kelas %s ini masih kosong, %s. Coba hubungi guru kamu dulu untuk menambahkan materi atau soal ya.", storedKelas, nama)
			}
		}
	}

	log.Printf("[tutor] job %s selesai: fase=%s balasan=%d karakter join=%v join_error=%q",
		jobID, out.Fase, len(out.Balasan), out.Join != nil, out.JoinError)
	return out, nil
}

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

	// HANYA gunakan kode dari ucapan user (last), JANGAN gunakan extract AI
	candidate := last
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

	konten := s.siswaService.HitungKontenKelas(siswa.KelasID)

	s.mu.Lock()
	s.sessionJoined[sessionID] = true
	s.sessionKelasNama[sessionID] = namaKelas
	s.sessionNama[sessionID] = siswa.Nama
	s.sessionKelasID[sessionID] = siswa.KelasID
	s.sessionKonten[sessionID] = konten
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

	switch {
	case konten.JumlahMateri > 0 && konten.JumlahSoal > 0:
		out.Balasan = fmt.Sprintf("Sempurna! Kamu sekarang resmi masuk kelas %s. Di kelas ini ada %d materi dan %d soal latihan yang bisa kamu pelajari. Hari ini kamu mau belajar apa, %s?",
			namaKelas, konten.JumlahMateri, konten.JumlahSoal, siswa.Nama)
	case konten.JumlahMateri > 0:
		out.Balasan = fmt.Sprintf("Sempurna! Kamu sekarang resmi masuk kelas %s. Di kelas ini tersedia %d materi bacaan. Mau kita mulai baca materi yang mana dulu, %s?",
			namaKelas, konten.JumlahMateri, siswa.Nama)
	case konten.JumlahSoal > 0:
		out.Balasan = fmt.Sprintf("Sempurna! Kamu sekarang resmi masuk kelas %s. Di kelas ini tersedia %d soal latihan. Mau kita mulai ngerjain soal yang mana dulu, %s?",
			namaKelas, konten.JumlahSoal, siswa.Nama)
	default:
		out.Balasan = fmt.Sprintf("Sempurna! Kamu sekarang resmi masuk kelas %s. Tapi sepertinya kelas ini masih kosong — belum ada materi atau soal. Coba hubungi guru kamu dulu ya, %s.",
			namaKelas, siswa.Nama)
	}

	log.Printf("[tutor] auto-join SUKSES: siswa %d (%s) kelas %d (%s) — konten: %d materi, %d soal",
		siswa.ID, siswa.Nama, siswa.KelasID, namaKelas, konten.JumlahMateri, konten.JumlahSoal)
}