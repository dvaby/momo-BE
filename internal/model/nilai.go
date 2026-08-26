package model

// NilaiSiswa merepresentasikan satu baris rekap nilai untuk seorang siswa
type NilaiSiswa struct {
	SiswaID           uint    `json:"siswa_id"`
	Nama              string  `json:"nama"`
	JumlahSoalDijawab int     `json:"jumlah_soal_dijawab"`
	JumlahBenar       int     `json:"jumlah_benar"`
	SkorPersen        float64 `json:"skor_persen"`
}