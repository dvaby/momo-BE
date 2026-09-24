package service

import (
	"log"
	"regexp"
	"strings"

	"gorm.io/gorm"

	"momo-be/internal/model"
)

// ========== Tipe state kuis ==========

// StateKuis melacak progres kuis suara dalam satu session
type StateKuis struct {
	Jenis     string     `json:"jenis"`
	SoalIDs   []uint     `json:"soal_ids"`
	Index     int        `json:"index"`
	Skor      int        `json:"skor"`
	Total     int        `json:"total"`
	SoalAktif *SoalAktif `json:"soal_aktif,omitempty"`
}

// SoalAktif adalah snapshot soal yang sedang ditampilkan ke user
// (disimpan di state biar backend tidak perlu query DB ulang tiap giliran)
type SoalAktif struct {
	Nomor      int    `json:"nomor"`
	Total      int    `json:"total"`
	Pertanyaan string `json:"pertanyaan"`
	PilihanA   string `json:"pilihan_a"`
	PilihanB   string `json:"pilihan_b"`
	PilihanC   string `json:"pilihan_c"`
	PilihanD   string `json:"pilihan_d"`
}

// NewSoalAktif membuat snapshot soal dari model.Soal
func NewSoalAktif(nomor, total int, s *model.Soal) *SoalAktif {
	return &SoalAktif{
		Nomor:      nomor,
		Total:      total,
		Pertanyaan: s.Pertanyaan,
		PilihanA:   s.PilihanA,
		PilihanB:   s.PilihanB,
		PilihanC:   s.PilihanC,
		PilihanD:   s.PilihanD,
	}
}

// ========== Service ==========

type KuisService struct {
	db *gorm.DB
}

func NewKuisService(db *gorm.DB) *KuisService {
	return &KuisService{db: db}
}

// InfoSoalKelas: jumlah soal per jenis untuk semua modul di kelas
func (s *KuisService) InfoSoalKelas(kelasID uint) []map[string]interface{} {
	type row struct {
		Jenis  string
		Jumlah int64
	}
	var rows []row
	s.db.Table("soals").
		Select("soals.jenis as jenis, count(*) as jumlah").
		Joins("JOIN moduls ON moduls.id = soals.modul_id").
		Joins("JOIN kelas_moduls ON kelas_moduls.modul_id = moduls.id").
		Where("kelas_moduls.kelas_id = ?", kelasID).
		Group("soals.jenis").
		Scan(&rows)

	out := make([]map[string]interface{}, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]interface{}{"jenis": r.Jenis, "jumlah": r.Jumlah})
	}
	return out
}

// AmbilSoalKelas: ambil soal per jenis (max limit) untuk sesi kuis suara
func (s *KuisService) AmbilSoalKelas(kelasID uint, jenis string, limit int) ([]model.Soal, error) {
	var soals []model.Soal
	err := s.db.Table("soals").
		Joins("JOIN moduls ON moduls.id = soals.modul_id").
		Joins("JOIN kelas_moduls ON kelas_moduls.modul_id = moduls.id").
		Where("kelas_moduls.kelas_id = ? AND soals.jenis = ?", kelasID, jenis).
		Order("soals.id ASC").
		Limit(limit).
		Find(&soals).Error
	return soals, err
}

// SoalByID ambil satu soal
func (s *KuisService) SoalByID(id uint) (*model.Soal, error) {
	var soal model.Soal
	if err := s.db.First(&soal, id).Error; err != nil {
		return nil, err
	}
	return &soal, nil
}

// SimpanJawaban persist ke tabel jawaban_siswas (untuk nilai di dashboard guru)
func (s *KuisService) SimpanJawaban(siswaID, soalID uint, mentah, terdeteksi string, benar bool, feedback string) error {
	rec := model.JawabanSiswa{
		SiswaID:           siswaID,
		SoalID:            soalID,
		JawabanMentah:     mentah,
		JawabanTerdeteksi: terdeteksi,
		Benar:             benar,
		Feedback:          feedback,
	}
	if err := s.db.Create(&rec).Error; err != nil {
		log.Printf("[kuis] gagal simpan jawaban: %v", err)
		return err
	}
	return nil
}

// ========== Helper deteksi & format ==========

var hurufDepanRe = regexp.MustCompile(`(?i)^(pilihan\s+|jawab\s+|huruf\s+|opsi\s+)?([a-d])\b`)

// DeteksiHuruf menebak huruf jawaban dari ucapan siswa ("b", "pilihan b", atau teks pilihan)
func DeteksiHuruf(jawaban string, soal *model.Soal) string {
	p := strings.ToLower(strings.TrimSpace(jawaban))
	if p == "" {
		return ""
	}
	if m := hurufDepanRe.FindStringSubmatch(p); len(m) > 2 {
		return strings.ToUpper(m[2])
	}
	pilihan := map[string]string{"A": soal.PilihanA, "B": soal.PilihanB, "C": soal.PilihanC, "D": soal.PilihanD}
	for _, h := range []string{"A", "B", "C", "D"} {
		t := strings.ToLower(strings.TrimSpace(pilihan[h]))
		if t == "" || len(p) < 4 {
			continue
		}
		if strings.Contains(p, t) || strings.Contains(t, p) {
			return h
		}
	}
	return ""
}

// FormatSoal teks soal siap dibacakan AI
func FormatSoal(nomor, total int, soal *model.Soal) map[string]interface{} {
	return map[string]interface{}{
		"nomor":      nomor,
		"total":      total,
		"pertanyaan": soal.Pertanyaan,
		"pilihan_a":  soal.PilihanA,
		"pilihan_b":  soal.PilihanB,
		"pilihan_c":  soal.PilihanC,
		"pilihan_d":  soal.PilihanD,
	}
}