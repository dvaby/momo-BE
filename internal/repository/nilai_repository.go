package repository

import (
	"gorm.io/gorm"
	"momo-be/internal/model"
)

type NilaiRepository struct {
	db *gorm.DB
}

func NewNilaiRepository(db *gorm.DB) *NilaiRepository {
	return &NilaiRepository{db: db}
}

// GetRekapNilai mengambil rekap nilai per siswa berdasarkan kelas, modul, dan jenis (opsional)
func (r *NilaiRepository) GetRekapNilai(kelasID uint, modulID uint, jenis string) ([]model.NilaiSiswa, error) {
	var result []model.NilaiSiswa

	// 1. Susun query dasar
	// Kita gunakan CTE (WITH) agar query kompleks ini lebih terstruktur dan mudah dibaca
	query := `
		WITH filtered_soal AS (
			SELECT id FROM soals
			WHERE modul_id = ?
	`
	args := []interface{}{modulID}

	// 2. Tambahkan filter jenis jika diisi (Opsional)
	if jenis != "" {
		query += ` AND jenis = ?`
		args = append(args, jenis)
	}

	// 3. Lanjutkan query utama
	// - DISTINCT ON memastikan kita hanya ambil 1 jawaban terbaru per siswa per soal
	// - NULLIF mencegah division by zero kalau ternyata belum ada soal sama sekali di modul tsb
	query += `
		),
		latest_answers AS (
			SELECT DISTINCT ON (js.siswa_id, js.soal_id)
				js.siswa_id,
				js.soal_id,
				js.benar
			FROM jawaban_siswas js
			INNER JOIN filtered_soal fs ON js.soal_id = fs.id
			ORDER BY js.siswa_id, js.soal_id, js.created_at DESC
		),
		total_soal_count AS (
			SELECT COUNT(*) AS total FROM filtered_soal
		)
		SELECT
			s.id AS siswa_id,
			s.nama,
			COUNT(la.soal_id) AS jumlah_soal_dijawab,
			COALESCE(SUM(CASE WHEN la.benar = true THEN 1 ELSE 0 END), 0) AS jumlah_benar,
			COALESCE(
				(COALESCE(SUM(CASE WHEN la.benar = true THEN 1 ELSE 0 END), 0)::float / 
				NULLIF((SELECT total FROM total_soal_count), 0)) * 100,
				0
			) AS skor_persen
		FROM siswas s
		LEFT JOIN latest_answers la ON s.id = la.siswa_id
		WHERE s.kelas_id = ?
		GROUP BY s.id, s.nama
		ORDER BY s.nama ASC
	`
	
	// Masukkan parameter terakhir untuk kelas_id
	args = append(args, kelasID)

	// Eksekusi Raw SQL dan petakan (scan) hasilnya ke slice of struct NilaiSiswa
	err := r.db.Raw(query, args...).Scan(&result).Error
	if err != nil {
		return nil, err
	}

	return result, nil
}