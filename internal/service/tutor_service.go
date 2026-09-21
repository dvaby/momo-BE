package service

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"regexp"
	"strings"
	"sync"
	"time"

	"momo-be/internal/job"
	"momo-be/pkg/aiclient"
)

// ========== Tipe data ==========

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
	ExtractNama string    `json:"extract_nama,omitempty"`
	ExtractKode string    `json:"extract_kode_kelas,omitempty"`
	Join        *JoinInfo `json:"join,omitempty"`
	JoinError   string    `json:"join_error,omitempty"`
}

// StateBelajar melacak progres pembacaan materi
type StateBelajar struct {
	Fase         string `json:"fase"`
	MateriID     uint   `json:"materi_id"`
	MateriJudul  string `json:"materi_judul"`
	MateriKonten string `json:"materi_konten"`
	ModulNama    string `json:"modul_nama"`
	ProgressBaca int    `json:"progress_baca"`
}

// ========== Helper package (pastikan hanya ADA DI FILE INI) ==========

var sixDigits = regexp.MustCompile(`\b\d{6}\b`)

func pilihAcak(opts []string) string {
	if len(opts) == 0 {
		return ""
	}
	return opts[rand.Intn(len(opts))]
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

// extractKodeKelas menormalkan noise STT menjadi 6 digit
func extractKodeKelas(pesan string, prevCode string) string {
	if m := sixDigits.FindString(pesan); m != "" {
		return m
	}
	var digits []rune
	for _, r := range pesan {
		if r >= '0' && r <= '9' {
			digits = append(digits, r)
		}
	}
	n := len(digits)
	if n == 0 {
		return ""
	}
	if n == 6 {
		return string(digits)
	}
	if n > 6 && n <= 18 {
		all := string(digits)
		first6 := all[:6]
		last6 := all[n-6:]
		if prevCode != "" {
			if last6 == prevCode {
				return last6
			}
			if first6 == prevCode {
				return first6
			}
		}
		return last6
	}
	return ""
}

// ========== Service ==========

type TutorService struct {
	aiClient     *aiclient.Client
	jobRegistry  *job.Registry
	callbackURL  string
	toolsExecURL string
	siswaService *SiswaService
	kuisService  *KuisService

	mu                 sync.Mutex
	sessionLastCode    map[string]string
	sessionFailedCode  map[string]string
	sessionJoined      map[string]bool
	sessionKelasNama   map[string]string
	sessionNama        map[string]string
	sessionKelasID     map[string]uint
	sessionKonten      map[string]KontenKelas
	sessionBelajar     map[string]*StateBelajar
	sessionKuis        map[string]*StateKuis
	sessionSiswaID     map[string]uint
	sessionBusy        map[string]bool
	sessionPendingJoin map[string]*JoinInfo
}

func NewTutorService(
	aiClient *aiclient.Client,
	jobRegistry *job.Registry,
	callbackURL string,
	toolsExecURL string,
	siswaService *SiswaService,
	kuisService *KuisService,
) *TutorService {
	return &TutorService{
		aiClient:           aiClient,
		jobRegistry:        jobRegistry,
		callbackURL:        callbackURL,
		toolsExecURL:       toolsExecURL,
		siswaService:       siswaService,
		kuisService:        kuisService,
		sessionLastCode:    make(map[string]string),
		sessionFailedCode:  make(map[string]string),
		sessionJoined:      make(map[string]bool),
		sessionKelasNama:   make(map[string]string),
		sessionNama:        make(map[string]string),
		sessionKelasID:     make(map[string]uint),
		sessionKonten:      make(map[string]KontenKelas),
		sessionBelajar:     make(map[string]*StateBelajar),
		sessionKuis:        make(map[string]*StateKuis),
		sessionSiswaID:     make(map[string]uint),
		sessionBusy:        make(map[string]bool),
		sessionPendingJoin: make(map[string]*JoinInfo),
	}
}

func (s *TutorService) currentFase(sessionID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessionJoined[sessionID] {
		return "belajar"
	}
	return "onboarding"
}

func (s *TutorService) buildSessionState(sessionID string) map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	return map[string]interface{}{
		"nama":          s.sessionNama[sessionID],
		"kode_kelas":    s.sessionLastCode[sessionID],
		"joined":        s.sessionJoined[sessionID],
		"kelas_nama":    s.sessionKelasNama[sessionID],
		"konten":        s.sessionKonten[sessionID],
		"state_belajar": s.sessionBelajar[sessionID],
	}
}

// ProcessTutor mode v2: backend tipis, AI yang jadi otak percakapan via function calling
func (s *TutorService) ProcessTutor(sessionID string, kelasNama string, pesanSiswa string) (*ChatResult, error) {
	jobID := job.GenerateID("tutor")

	// Guard 1: pesan kosong (mic tidak menangkap suara)
	if strings.TrimSpace(pesanSiswa) == "" {
		return &ChatResult{
			JobID:   jobID,
			Balasan: pilihAcak([]string{"Hmm, aku tidak mendengar suaramu. Bisa diulang lagi?", "Eh, suaramu tidak masuk. Coba bilang lagi ya."}),
			Fase:    s.currentFase(sessionID),
		}, nil
	}

	// Guard 2: anti double-send — satu session hanya boleh punya 1 job berjalan
	s.mu.Lock()
	if s.sessionBusy[sessionID] {
		s.mu.Unlock()
		return &ChatResult{
			JobID:   jobID,
			Balasan: "Tunggu sebentar ya, aku masih selesai bicara.",
			Fase:    s.currentFase(sessionID),
		}, nil
	}
	s.sessionBusy[sessionID] = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.sessionBusy, sessionID)
		s.mu.Unlock()
	}()

	s.jobRegistry.Register(jobID, job.JobTypeTutor, 0, "", sessionID)

	payload := map[string]interface{}{
		"job_id":         jobID,
		"callback_url":   s.callbackURL,
		"tools_exec_url": s.toolsExecURL,
		"session_id":     sessionID,
		"pesan":          pesanSiswa,
		"mode":           "function_calling",
		"state":          s.buildSessionState(sessionID),
		"tools":          GetToolDefinitions(),
	}

	if err := s.aiClient.SubmitTutorV2(payload); err != nil {
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

	var hasil struct {
		Balasan string `json:"balasan"`
		Fase    string `json:"fase"`
	}
	if err := json.Unmarshal(finished.Hasil, &hasil); err != nil {
		return nil, fmt.Errorf("gagal parse balasan tutor: %w", err)
	}

	out := &ChatResult{
		JobID:   jobID,
		Balasan: strings.TrimSpace(hasil.Balasan),
		Fase:    hasil.Fase,
	}

	// Lampirkan join yang terjadi di giliran ini (hasil tool join_kelas)
	s.mu.Lock()
	pending := s.sessionPendingJoin[sessionID]
	delete(s.sessionPendingJoin, sessionID)
	joined := s.sessionJoined[sessionID]
	s.mu.Unlock()
	if pending != nil {
		out.Join = pending
		out.Fase = "belajar"
		log.Printf("[tutor] join dilampirkan ke response: siswa %d kelas %s", pending.SiswaID, pending.KelasNama)
	}

	// Safety net minimal
	if out.Balasan == "" {
		out.Balasan = pilihAcak([]string{"Maaf, aku tadi sempat blank. Bisa diulang lagi?", "Eh, ucapanku tadi belum keluar. Kamu bilang apa?"})
	}
	if joined && out.Fase != "belajar" {
		out.Fase = "belajar"
	}
	if !joined && out.Fase == "belajar" {
		out.Fase = "onboarding"
	}

	log.Printf("[tutor] job %s selesai: fase=%s balasan=%d karakter join=%v",
		jobID, out.Fase, len(out.Balasan), out.Join != nil)
	return out, nil
}