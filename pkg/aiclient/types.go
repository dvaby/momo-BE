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
	Success bool                 `json:"success"`
	Data    ProcessResponseData  `json:"data"`
}