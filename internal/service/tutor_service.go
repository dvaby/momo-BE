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

// StateBelajar melacak progres pembelajaran siswa
type StateBelajar struct {
	Fase              string `json:"fase"` // "pilih_materi", "konfirmasi_materi", "siap_dengar", "baca_setengah", "tanya_paham", "lanjut_atau_ulang"
	MateriID          uint   `json:"materi_id"`
	MateriJudul       string `json:"materi_judul"`
	MateriKonten      string `json:"materi_konten"`
	ModulNama         string `json:"modul_nama"`
	ProgressBaca      int    `json:"progress_baca"` // 0 = belum mulai, 50 = setengah, 100 = selesai
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
	sessionBelajar    map[string]*StateBelajar // NEW: state pembelajaran per session
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
		sessionBelajar:    make(map[string]*StateBelajar),
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

// deteksiPilihanMateri mengecek apakah user menyebut nomor atau judul materi
func deteksiPilihanMateri(pesan string, materiList []map[string]interface{}) (uint, string) {
	p := strings.ToLower(strings.TrimSpace(pesan))
	
	// Cek apakah user menyebut angka (nomor materi)
	numRe := regexp.MustCompile(`\b(\d+)\b`)
	if matches := numRe.FindStringSubmatch(p); len(matches) > 1 {
		idx := 0
		fmt.Sscanf(matches[1], "%d", &idx)
		if idx > 0 && idx <= len(materiList) {
			materi := materiList[idx-1]
			return materi["id"].(uint), materi["judul"].(string)
		}
	}

	// Cek apakah user menyebut judul materi
	for _, materi := range materiList {
		judul := strings.ToLower(materi["judul"].(string))
		if strings.Contains(p, judul) || strings.Contains(judul, p) {
			return materi["id"].(uint), materi["judul"].(string)
		}
	}

	return 0, ""
}

func (s *TutorService) ProcessTutor(sessionID string, kelasNama string, pesanSiswa string) (*ChatResult, error) {
	jobID := job.GenerateID("tutor")
	s.jobRegistry.Register(jobID, job.JobTypeTutor, 0, "", sessionID)

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
	stateBelajar := s.sessionBelajar[sessionID]
	s.mu.Unlock()

	konteks := fmt.Sprintf("Kelas: %s | Session: %s", kelasNama, sessionID)
	if alreadyJoined {
		konteks += fmt.Sprintf(" | STATUS: ONBOARDING SELESAI. Siswa sudah terdaftar di kelas %s.", storedKelas)
		konteks += fmt.Sprintf(" | KONTEN KELAS: %d materi, %d soal tersedia.", storedKonten.JumlahMateri, storedKonten.JumlahSoal)
		
		if stateBelajar != nil {
			konteks += fmt.Sprintf(" | STATE BELAJAR: fase=%s, materi_id=%d, judul=%s, progress=%d%%.", 
				stateBelajar.Fase, stateBelajar.MateriID, stateBelajar.MateriJudul, stateBelajar.ProgressBaca)
		}
		
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

	mintaKodePositif := strings.Contains(balasanLower, "sebutkan kode") ||
		strings.Contains(balasanLower, "sebutkan kembali kode") ||
		strings.Contains(balasanLower, "bisa sebutkan lagi") ||
		strings.Contains(balasanLower, "sebutkan lagi kode") ||
		strings.Contains(balasanLower, "belum terkonfirmasi") ||
		strings.Contains(balasanLower, "mohon sebutkan kode") ||
		strings.Contains(balasanLower, "kurang mendengar kode") ||
		strings.Contains(balasanLower, "tidak terdengar jelas") ||
		(strings.Contains(balasanLower, "kode kelas") && strings.Contains(balasanLower, "sebutkan lagi"))

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
		case isKonfirmasi && lastCode != "":
			s.handleAutoJoin(sessionID, out)
		}

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
		// SUDAH JOIN: MODE BELAJAR DENGAN STATE MACHINE
		out.Fase = "belajar"

		niat, topik := deteksiNiatUser(pesanSiswa)
		_ = storedKelasID

		nama := storedNama
		if nama == "" {
			nama = "Siswa"
		}

		// STATE MACHINE: Alur Pembelajaran Interaktif
		if stateBelajar != nil {
			// Ada state belajar aktif, handle sesuai fase
			switch stateBelajar.Fase {
			case "pilih_materi":
				// User sedang memilih materi dari list
				materiList, _ := s.siswaService.GetDaftarMateri(storedKelasID)
				materiID, materiJudul := deteksiPilihanMateri(pesanSiswa, materiList)
				
				if materiID > 0 {
					// User memilih materi, ambil konten lengkap
					materi, modulNama, _ := s.siswaService.GetMateriByID(materiID)
					if materi != nil {
						s.mu.Lock()
						s.sessionBelajar[sessionID] = &StateBelajar{
							Fase:         "konfirmasi_materi",
							MateriID:     materiID,
							MateriJudul:  materiJudul,
							MateriKonten: materi.Konten,
							ModulNama:    modulNama,
							ProgressBaca: 0,
						}
						s.mu.Unlock()
						
						out.Balasan = fmt.Sprintf("Oke %s, kamu memilih materi '%s' dari modul %s. Apakah kamu siap mendengarkan materinya?", 
							nama, materiJudul, modulNama)
						log.Printf("[tutor] state: pilih_materi -> konfirmasi_materi (materi_id=%d)", materiID)
					}
				} else {
					out.Balasan = fmt.Sprintf("Maaf %s, aku tidak mengerti pilihanmu. Bisa sebutkan nomor atau judul materi yang kamu pilih?", nama)
				}

			case "konfirmasi_materi":
				// User konfirmasi siap mendengarkan
				if isKonfirmasi {
					s.mu.Lock()
					s.sessionBelajar[sessionID].Fase = "baca_setengah"
					s.sessionBelajar[sessionID].ProgressBaca = 50
					s.mu.Unlock()
					
					// Baca setengah materi
					konten := stateBelajar.MateriKonten
					setengahKonten := konten[:len(konten)/2]
					
					out.Balasan = fmt.Sprintf("Baik, aku akan bacakan materi '%s'. Ini bagian pertama:\n\n%s\n\nOke, sudah setengah jalan. Apakah kamu paham dengan materi yang aku bacakan?", 
						stateBelajar.MateriJudul, setengahKonten)
					log.Printf("[tutor] state: konfirmasi_materi -> baca_setengah (progress=50%%)")
				} else {
					out.Balasan = fmt.Sprintf("Oke %s, kalau belum siap, bilang saja kalau kamu sudah siap ya.", nama)
				}

			case "baca_setengah":
				// User jawab apakah paham atau tidak
				if strings.Contains(pesanLower, "paham") || strings.Contains(pesanLower, "mengerti") {
					if negasiRe.MatchString(pesanLower) {
						// User tidak paham -> ulang
						s.mu.Lock()
						s.sessionBelajar[sessionID].Fase = "konfirmasi_materi"
						s.sessionBelajar[sessionID].ProgressBaca = 0
						s.mu.Unlock()
						
						out.Balasan = fmt.Sprintf("Oke %s, tidak apa-apa. Aku akan ulang dari awal ya. Apakah kamu siap mendengarkan lagi?", nama)
						log.Printf("[tutor] state: baca_setengah -> konfirmasi_materi (ulang)")
					} else {
						// User paham -> lanjut ke sisa materi
						s.mu.Lock()
						s.sessionBelajar[sessionID].Fase = "lanjut_atau_ulang"
						s.sessionBelajar[sessionID].ProgressBaca = 100
						s.mu.Unlock()
						
						konten := stateBelajar.MateriKonten
						sisaKonten := konten[len(konten)/2:]
						
						out.Balasan = fmt.Sprintf("Bagus! Sekarang aku lanjutkan bagian kedua:\n\n%s\n\nNah, sekarang materinya sudah selesai. Ada yang ingin kamu tanyakan atau kita lanjut ke materi lain?", 
							sisaKonten)
						log.Printf("[tutor] state: baca_setengah -> lanjut_atau_ulang (progress=100%%)")
					}
				} else {
					out.Balasan = fmt.Sprintf("Maaf %s, aku tidak mengerti. Apakah kamu paham dengan materi yang aku bacakan? Jawab 'ya paham' atau 'belum paham'.", nama)
				}

			case "lanjut_atau_ulang":
				// Materi selesai, user bisa pilih materi lain atau tanya sesuatu
				// Reset state
				s.mu.Lock()
				delete(s.sessionBelajar, sessionID)
				s.mu.Unlock()
				
				// Biarkan AI handle natural conversation
				if niat == "materi" {
					out.Balasan = fmt.Sprintf("Oke %s, mau belajar materi apa lagi? Di kelas %s ini ada %d materi yang bisa kamu pelajari.", 
						nama, storedKelas, storedKonten.JumlahMateri)
				}
				log.Printf("[tutor] state: lanjut_atau_ulang -> reset (materi selesai)")

			default:
				// State tidak dikenal, reset
				s.mu.Lock()
				delete(s.sessionBelajar, sessionID)
				s.mu.Unlock()
			}
		} else if niat == "materi" && storedKonten.JumlahMateri > 0 {
			// User minta materi tapi belum ada state -> tampilkan list materi
			materiList, _ := s.siswaService.GetDaftarMateri(storedKelasID)
			
			if len(materiList) > 0 {
				listMateri := ""
				for i, materi := range materiList {
					listMateri += fmt.Sprintf("%d. %s (%s)\n", i+1, materi["judul"], materi["modul"])
				}
				
				s.mu.Lock()
				s.sessionBelajar[sessionID] = &StateBelajar{
					Fase: "pilih_materi",
				}
				s.mu.Unlock()
				
				out.Balasan = fmt.Sprintf("Ini beberapa materi yang tersedia di kelas %s:\n\n%s\nKamu mau pilih yang mana? Sebutkan nomor atau judul materinya ya.", 
					storedKelas, listMateri)
				log.Printf("[tutor] state: new -> pilih_materi (tampilkan %d materi)", len(materiList))
			} else {
				out.Balasan = fmt.Sprintf("Hmm, di kelas %s ini belum ada materi bacaan, %s. Yang tersedia baru %d soal latihan. Mau kita mulai ngerjain soal dulu?",
					storedKelas, nama, storedKonten.JumlahSoal)
			}
		} else if niat == "soal" && storedKonten.JumlahSoal == 0 && storedKonten.JumlahMateri > 0 {
			out.Balasan = fmt.Sprintf("Hmm, di kelas %s ini belum ada soal latihan, %s. Yang tersedia baru %d materi bacaan. Mau kita mulai baca materi dulu?",
				storedKelas, nama, storedKonten.JumlahMateri)
		} else if cekKataOnboarding(balasanLower) {
			// Override agresif untuk kata onboarding
			if len(topik) > 2 {
				out.Balasan = fmt.Sprintf("Oke %s! Ayo kita bahas %s. Apa yang ingin kamu ketahui dulu tentang itu?", nama, topik)
			} else {
				out.Balasan = fmt.Sprintf("Kamu sudah masuk kelas %s, %s. Tidak perlu kode atau nama lagi. Hari ini kamu mau belajar apa? Di kelas ini ada %d materi dan %d soal yang bisa kamu pelajari.",
					storedKelas, nama, storedKonten.JumlahMateri, storedKonten.JumlahSoal)
			}
			log.Printf("[tutor] override agresif: AI menyebut kata onboarding pasca-join (session %s)", sessionID)
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