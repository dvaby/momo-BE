package job

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// JobType merepresentasikan tipe job AI
type JobType string

const (
	JobTypeMateri   JobType = "materi"
	JobTypeSoal     JobType = "soal"
	JobTypeEvaluate JobType = "evaluate"
	JobTypeTutor    JobType = "tutor"
)

// Job merepresentasikan satu unit kerja async ke AI Service
type Job struct {
	ID         string
	Type       JobType
	ModulID    uint   // untuk materi/soal
	Jenis      string // untuk soal (harian/uts/uas)
	SessionID  string // untuk evaluate/tutor
	CreatedAt  time.Time
	CallbackAt time.Time
	Status     string // pending | success | failed
	Hasil      []byte // JSON mentah hasil callback
	Error      string
	Processed  bool // idempotency flag
}

// ============================================================
// TIPE KONTRAK CALLBACK (AI Service → Backend)
// ============================================================

// CallbackRequest adalah body yang dikirim AI Service ke backend
// setelah selesai memproses job.
type CallbackRequest struct {
	JobID        string          `json:"job_id"`
	Tipe         string          `json:"tipe"`   // materi | soal | evaluate | tutor
	Status       string          `json:"status"` // success | failed
	ErrorMessage string          `json:"error_message,omitempty"`
	Hasil        json.RawMessage `json:"hasil,omitempty"`
}

// HasilProcess adalah bentuk terstruktur dari hasil /process (materi atau soal).
type HasilProcess struct {
	Materi []MateriItem `json:"materi,omitempty"`
	Soal   []SoalItem   `json:"soal,omitempty"`
}

type MateriItem struct {
	Urutan int    `json:"urutan"`
	Judul  string `json:"judul"`
	Konten string `json:"konten"`
}

type SoalItem struct {
	Pertanyaan   string `json:"pertanyaan"`
	PilihanA     string `json:"pilihan_a"`
	PilihanB     string `json:"pilihan_b"`
	PilihanC     string `json:"pilihan_c"`
	PilihanD     string `json:"pilihan_d"`
	KunciJawaban string `json:"kunci_jawaban"`
}

// HasilEvaluate adalah bentuk terstruktur dari hasil /evaluate.
type HasilEvaluate struct {
	JawabanTerdeteksi string `json:"jawaban_terdeteksi"`
	Benar             bool   `json:"benar"`
	Feedback          string `json:"feedback"`
	PerluKlarifikasi  bool   `json:"perlu_klarifikasi,omitempty"`
}

// HasilTutor adalah bentuk terstruktur dari hasil /tutor.
type HasilTutor struct {
	Balasan string `json:"balasan"`
}

// ============================================================
// CALLBACK LISTENER & REGISTRY
// ============================================================

// CallbackListener adalah fungsi yang dipanggil saat callback diterima
// untuk job tertentu (evaluate sync-via-callback).
type CallbackListener func(job *Job)

// Registry menyimpan state semua job aktif di memory.
// Idempotent by job_id; aman diakses dari banyak goroutine.
type Registry struct {
	mu        sync.RWMutex
	jobs      map[string]*Job
	listeners map[string]CallbackListener // job_id -> listener (untuk evaluate sync)
}

func NewRegistry() *Registry {
	return &Registry{
		jobs:      make(map[string]*Job),
		listeners: make(map[string]CallbackListener),
	}
}

// Register mendaftarkan job baru. Idempotent: jika job_id sudah ada, abaikan.
func (r *Registry) Register(id string, jobType JobType, modulID uint, jenis string, sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.jobs[id]; exists {
		return
	}
	r.jobs[id] = &Job{
		ID:        id,
		Type:      jobType,
		ModulID:   modulID,
		Jenis:     jenis,
		SessionID: sessionID,
		CreatedAt: time.Now(),
		Status:    "pending",
	}
}

// RegisterListener memasang listener yang akan dipanggil saat callback untuk job_id diterima.
// Berguna untuk evaluate sync-via-callback: handler subscribe, callback endpoint notify.
func (r *Registry) RegisterListener(jobID string, fn CallbackListener) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.listeners[jobID] = fn
}

// WaitSync menunggu callback untuk job_id (blocking) sampai timeout.
// Return: job final state atau nil jika timeout.
func (r *Registry) WaitSync(jobID string, timeout time.Duration) *Job {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			r.mu.RLock()
			j, ok := r.jobs[jobID]
			r.mu.RUnlock()
			if ok && j.Status != "pending" {
				return j
			}
		}
	}
}

// Complete menandai job sukses dengan hasil (idempotent).
func (r *Registry) Complete(jobID string, hasil []byte) error {
	r.mu.Lock()
	j, ok := r.jobs[jobID]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("job_id tidak ditemukan: %s", jobID)
	}
	if j.Processed {
		r.mu.Unlock()
		return nil // idempotent: abaikan duplikat
	}
	j.Status = "success"
	j.CallbackAt = time.Now()
	j.Hasil = hasil
	j.Processed = true
	// Ambil listener sebelum unlock
	listener := r.listeners[jobID]
	delete(r.listeners, jobID)
	r.mu.Unlock()

	if listener != nil {
		listener(j)
	}
	return nil
}

// Fail menandai job gagal (idempotent).
func (r *Registry) Fail(jobID string, errMsg string) error {
	r.mu.Lock()
	j, ok := r.jobs[jobID]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("job_id tidak ditemukan: %s", jobID)
	}
	if j.Processed {
		r.mu.Unlock()
		return nil
	}
	j.Status = "failed"
	j.CallbackAt = time.Now()
	j.Error = errMsg
	j.Processed = true
	listener := r.listeners[jobID]
	delete(r.listeners, jobID)
	r.mu.Unlock()

	if listener != nil {
		listener(j)
	}
	return nil
}

// Get mengambil state job (read-only copy).
func (r *Registry) Get(jobID string) *Job {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if j, ok := r.jobs[jobID]; ok {
		copy := *j
		return &copy
	}
	return nil
}

// Cleanup menghapus job yang sudah selesai lebih dari retention.
// Panggil periodik (misal tiap 5 menit) untuk mencegah memory leak.
func (r *Registry) Cleanup(retention time.Duration) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	cutoff := time.Now().Add(-retention)
	removed := 0
	for id, j := range r.jobs {
		if j.Processed && j.CallbackAt.Before(cutoff) {
			delete(r.jobs, id)
			removed++
		}
	}
	return removed
}
