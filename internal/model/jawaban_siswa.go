package model

import "time"

// JawabanSiswa menyimpan setiap percobaan siswa menjawab satu Soal,
// beserta hasil evaluasi dari AI Service. Untuk soal jenis "harian",
// siswa boleh submit berkali-kali (tiap percobaan tersimpan sebagai
// baris baru). Untuk soal jenis "uts"/"uas", hanya boleh 1 baris per
// SiswaID+SoalID — aturan ini ditegakkan di service layer, bukan di
// level model/database.
type JawabanSiswa struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	SiswaID            uint      `json:"siswa_id"`
	SoalID             uint      `json:"soal_id"`
	JawabanMentah      string    `json:"jawaban_mentah"`
	JawabanTerdeteksi  string    `json:"jawaban_terdeteksi"`
	Benar              bool      `json:"benar"`
	Feedback           string    `json:"feedback"`
	CreatedAt          time.Time `json:"created_at"`
}