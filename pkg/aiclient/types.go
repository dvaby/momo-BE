package aiclient

// ============================================================
// TIPE EXISTING (tidak berubah — backward compatible)
// ============================================================

type ProcessRequest struct {
	Tipe       string `json:"tipe"`
	TeksMentah string `json:"teks_mentah"`
	// Field baru untuk arsitektur v1.4+
	JobID       string `json:"job_id,omitempty"`
	CallbackURL string `json:"callback_url,omitempty"`
	Konteks     string `json:"konteks,omitempty"`
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

type ProcessResponseData struct {
	Materi []MateriItem `json:"materi,omitempty"`
	Soal   []SoalItem   `json:"soal,omitempty"`
}

type ProcessResponse struct {
	Success bool                `json:"success"`
	Message string              `json:"message,omitempty"`
	Data    ProcessResponseData `json:"data"`
}

type EvaluateRequest struct {
	Pertanyaan         string `json:"pertanyaan"`
	PilihanA           string `json:"pilihan_a"`
	PilihanB           string `json:"pilihan_b"`
	PilihanC           string `json:"pilihan_c"`
	PilihanD           string `json:"pilihan_d"`
	KunciJawaban       string `json:"kunci_jawaban"`
	JawabanSiswaMentah string `json:"jawaban_siswa_mentah"`
	// Field baru untuk arsitektur v1.4+
	JobID       string         `json:"job_id,omitempty"`
	CallbackURL string         `json:"callback_url,omitempty"`
	SessionID   string         `json:"session_id,omitempty"`
	Bootstrap   *BootstrapData `json:"bootstrap,omitempty"`
	Konteks     string         `json:"konteks,omitempty"`
}

type BootstrapData struct {
	RiwayatTerakhir []RiwayatItem `json:"riwayat_terakhir,omitempty"`
	MateriAktif     *MateriInfo   `json:"materi_aktif,omitempty"`
}

type RiwayatItem struct {
	SoalID uint   `json:"soal_id"`
	Benar  bool   `json:"benar"`
	Konsep string `json:"konsep,omitempty"`
}

type MateriInfo struct {
	Judul     string `json:"judul,omitempty"`
	Ringkasan string `json:"ringkasan,omitempty"`
}

type EvaluateResponseData struct {
	JawabanTerdeteksi string `json:"jawaban_terdeteksi"`
	Benar             bool   `json:"benar"`
	Feedback          string `json:"feedback"`
	PerluKlarifikasi  bool   `json:"perlu_klarifikasi,omitempty"`
}

type EvaluateResponse struct {
	Success bool                 `json:"success"`
	Message string               `json:"message,omitempty"`
	Data    EvaluateResponseData `json:"data"`
}

// ============================================================
// TIPE BARU untuk arsitektur v1.4+ (hanya JobAck untuk deteksi dual-mode)
// ============================================================

// JobAck adalah response AI ketika menerima job (pola baru).
// Jika AI mengembalikan ini, backend harus menunggu callback.
type JobAck struct {
	Accepted bool   `json:"accepted"`
	JobID    string `json:"job_id"`
}

// IsJobAck memeriksa apakah response adalah ack (pola baru) atau hasil (pola lama).
func (j *JobAck) IsJobAck() bool {
	return j != nil && j.Accepted && j.JobID != ""
}
