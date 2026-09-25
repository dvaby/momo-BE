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
	"momo-be/internal/repository"
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

type StateBelajar struct {
	Fase         string `json:"fase"`
	MateriID     uint   `json:"materi_id"`
	MateriJudul  string `json:"materi_judul"`
	MateriKonten string `json:"materi_konten"`
	ModulNama    string `json:"modul_nama"`
	ProgressBaca int    `json:"progress_baca"`
}

// SessionData adalah seluruh state percakapan yang dipersistenkan ke DB
type SessionData struct {
	Nama        string        `json:"nama"`
	LastCode    string        `json:"last_code"`
	FailedCode  string        `json:"failed_code"`
	Joined      bool          `json:"joined"`
	KelasNama   string        `json:"kelas_nama"`
	KelasID     uint          `json:"kelas_id"`
	SiswaID     uint          `json:"siswa_id"`
	Konten      KontenKelas   `json:"konten"`
	Belajar     *StateBelajar `json:"belajar"`
	Kuis        *StateKuis    `json:"kuis"`
	PendingJoin *JoinInfo     `json:"pending_join,omitempty"`
}

// ========== Helper package (hanya ADA DI FILE INI) ==========

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
	repo         repository.SessionStateRepository
	progressRepo *repository.ProgressRepository

	mu          sync.Mutex
	cache       map[string]*SessionData // cache in-memory (DB = sumber kebenaran)
	sessionBusy map[string]bool
}

func NewTutorService(
	aiClient *aiclient.Client,
	jobRegistry *job.Registry,
	callbackURL string,
	toolsExecURL string,
	siswaService *SiswaService,
	kuisService *KuisService,
	repo repository.SessionStateRepository,
	progressRepo *repository.ProgressRepository,
) *TutorService {
	return &TutorService{
		aiClient:     aiClient,
		jobRegistry:  jobRegistry,
		callbackURL:  callbackURL,
		toolsExecURL: toolsExecURL,
		siswaService: siswaService,
		kuisService:  kuisService,
		repo:         repo,
		progressRepo: progressRepo,
		cache:        make(map[string]*SessionData),
		sessionBusy:  make(map[string]bool),
	}
}

// loadState: cache dulu, kalau miss ambil dari DB (cache-aside)
func (s *TutorService) loadState(sessionID string) *SessionData {
	s.mu.Lock()
	if st, ok := s.cache[sessionID]; ok {
		s.mu.Unlock()
		return st
	}
	s.mu.Unlock()

	st := &SessionData{}
	if jsonStr, err := s.repo.Get(sessionID); err == nil && jsonStr != "" {
		if err := json.Unmarshal([]byte(jsonStr), st); err != nil {
			log.Printf("[session] gagal unmarshal %s: %v", sessionID, err)
			st = &SessionData{}
		}
	}
	s.mu.Lock()
	s.cache[sessionID] = st
	s.mu.Unlock()
	return st
}

// saveState: write-through ke cache + DB
func (s *TutorService) saveState(sessionID string, st *SessionData) {
	s.mu.Lock()
	s.cache[sessionID] = st
	s.mu.Unlock()
	b, err := json.Marshal(st)
	if err != nil {
		log.Printf("[session] gagal marshal %s: %v", sessionID, err)
		return
	}
	if err := s.repo.Save(sessionID, string(b)); err != nil {
		log.Printf("[session] gagal save ke DB %s: %v", sessionID, err)
	}
}

func (s *TutorService) deleteState(sessionID string) {
	s.mu.Lock()
	delete(s.cache, sessionID)
	s.mu.Unlock()
	_ = s.repo.Delete(sessionID)
}

func (s *TutorService) currentFase(sessionID string) string {
	if s.loadState(sessionID).Joined {
		return "belajar"
	}
	return "onboarding"
}

func (s *TutorService) buildSessionState(sessionID string) map[string]interface{} {
	st := s.loadState(sessionID)
	payload := map[string]interface{}{
		"nama":          st.Nama,
		"kode_kelas":    st.LastCode,
		"joined":        st.Joined,
		"kelas_nama":    st.KelasNama,
		"konten":        st.Konten,
		"state_belajar": st.Belajar,
	}
	if st.Kuis != nil {
		payload["state_kuis"] = map[string]interface{}{
			"jenis":      st.Kuis.Jenis,
			"index":      st.Kuis.Index,
			"total":      st.Kuis.Total,
			"skor":       st.Kuis.Skor,
			"soal_aktif": st.Kuis.SoalAktif,
		}
		log.Printf("[tutor] state_kuis dikirim ke AI: jenis=%s index=%d total=%d skor=%d",
			st.Kuis.Jenis, st.Kuis.Index, st.Kuis.Total, st.Kuis.Skor)
	}
	return payload
}

func (s *TutorService) ProcessTutor(sessionID string, kelasNama string, pesanSiswa string) (*ChatResult, error) {
	jobID := job.GenerateID("tutor")

	if strings.TrimSpace(pesanSiswa) == "" {
		return &ChatResult{
			JobID:   jobID,
			Balasan: pilihAcak([]string{"Hmm, aku tidak mendengar suaramu. Bisa diulang lagi?", "Eh, suaramu tidak masuk. Coba bilang lagi ya."}),
			Fase:    s.currentFase(sessionID),
		}, nil
	}

	// Guard anti double-send
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

	// Lampirkan join yang terjadi giliran ini, lalu bersihkan
	st := s.loadState(sessionID)
	if st.PendingJoin != nil {
		out.Join = st.PendingJoin
		out.Fase = "belajar"
		st.PendingJoin = nil
		s.saveState(sessionID, st)
		log.Printf("[tutor] join dilampirkan ke response: siswa %d kelas %s", st.SiswaID, st.KelasNama)
	}

	if out.Balasan == "" {
		out.Balasan = pilihAcak([]string{"Maaf, aku tadi sempat blank. Bisa diulang lagi?", "Eh, ucapanku tadi belum keluar. Kamu bilang apa?"})
	}
	if st.Joined && out.Fase != "belajar" {
		out.Fase = "belajar"
	}
	if !st.Joined && out.Fase == "belajar" {
		out.Fase = "onboarding"
	}

	log.Printf("[tutor] job %s selesai: fase=%s balasan=%d karakter join=%v",
		jobID, out.Fase, len(out.Balasan), out.Join != nil)
	return out, nil
}