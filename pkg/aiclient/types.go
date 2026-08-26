package aiclient

type ProcessRequest struct {
	Tipe       string `json:"tipe"`
	TeksMentah string `json:"teks_mentah"`
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

// EvaluateRequest adalah body yang dikirim ke AI Service untuk menilai
// jawaban siswa. jawaban_siswa_mentah sengaja bertipe teks bebas karena
// berasal dari hasil Speech-to-Text, bukan huruf pasti A/B/C/D.
type EvaluateRequest struct {
	Pertanyaan         string `json:"pertanyaan"`
	PilihanA           string `json:"pilihan_a"`
	PilihanB           string `json:"pilihan_b"`
	PilihanC           string `json:"pilihan_c"`
	PilihanD           string `json:"pilihan_d"`
	KunciJawaban       string `json:"kunci_jawaban"`
	JawabanSiswaMentah string `json:"jawaban_siswa_mentah"`
}

// EvaluateResponseData adalah hasil evaluasi dari AI Service.
type EvaluateResponseData struct {
	JawabanTerdeteksi string `json:"jawaban_terdeteksi"`
	Benar             bool   `json:"benar"`
	Feedback          string `json:"feedback"`
}

type EvaluateResponse struct {
	Success bool                  `json:"success"`
	Message string                `json:"message,omitempty"`
	Data    EvaluateResponseData  `json:"data"`
}