package service

import (
	"encoding/json"
	"strconv"
	"time"

	"gorm.io/gorm"

	"momo-be/internal/model"
)

type ProgressService struct{ db *gorm.DB }

func NewProgressService(db *gorm.DB) *ProgressService { return &ProgressService{db: db} }

type ProgressSiswaDTO struct {
	SiswaID           uint    `json:"siswa_id"`
	Nama              string  `json:"nama"`
	ProgressMateri    string  `json:"progress_materi"`
	PersenMateri      float64 `json:"persen_materi"`
	MateriSelesai     int     `json:"materi_selesai"`
	SoalDikerjakan    int     `json:"soal_dikerjakan"`
	NilaiRata         float64 `json:"nilai_rata"`
	AktivitasTerakhir string  `json:"aktivitas_terakhir"`
	Status            string  `json:"status"` // aman | perlu_perhatian | belum_aktif
}

type ProgressRingkasanDTO struct {
	TotalSiswa     int     `json:"total_siswa"`
	RataNilai      float64 `json:"rata_nilai"`
	MateriSelesai  int     `json:"materi_selesai"`
	PerluPerhatian int     `json:"perlu_perhatian"`
}

func (s *ProgressService) GetProgressKelas(kelasID uint) (ProgressRingkasanDTO, []ProgressSiswaDTO) {
	ring := ProgressRingkasanDTO{}
	out := []ProgressSiswaDTO{}

	var siswas []model.Siswa
	if err := s.db.Where("kelas_id = ?", kelasID).Order("id ASC").Find(&siswas).Error; err != nil {
		return ring, out
	}
	ring.TotalSiswa = len(siswas)
	if len(siswas) == 0 {
		return ring, out
	}
	ids := make([]uint, 0, len(siswas))
	for _, sw := range siswas {
		ids = append(ids, sw.ID)
	}

	var pros []model.SiswaProgress
	s.db.Where("siswa_id IN ?", ids).Find(&pros)
	progMap := map[uint]model.SiswaProgress{}
	for _, p := range pros {
		progMap[p.SiswaID] = p
	}

	type agg struct {
		SiswaID uint
		Total   int64
		Benar   int64
	}
	var aggs []agg
	s.db.Model(&model.JawabanSiswa{}).
		Select("siswa_id, count(*) as total, sum(case when benar then 1 else 0 end) as benar").
		Where("siswa_id IN ?", ids).
		Group("siswa_id").Scan(&aggs)
	aggMap := map[uint]agg{}
	for _, a := range aggs {
		aggMap[a.SiswaID] = a
	}

	var totalMateri int64
	s.db.Table("materis").
		Joins("JOIN moduls ON moduls.id = materis.modul_id").
		Joins("JOIN kelas_moduls ON kelas_moduls.modul_id = moduls.id").
		Where("kelas_moduls.kelas_id = ?", kelasID).
		Count(&totalMateri)

	sumNilai, countNilai := 0.0, 0
	for _, sw := range siswas {
		dto := ProgressSiswaDTO{SiswaID: sw.ID, Nama: sw.Nama, AktivitasTerakhir: "-"}

		var materiIDs []uint
		if p, ok := progMap[sw.ID]; ok {
			_ = json.Unmarshal([]byte(p.MateriSelesai), &materiIDs)
			if !p.LastActivity.IsZero() {
				dto.AktivitasTerakhir = p.LastActivity.Format(time.RFC3339)
			}
		}
		dto.MateriSelesai = len(materiIDs)
		if totalMateri > 0 {
			dto.PersenMateri = float64(len(materiIDs)) / float64(totalMateri) * 100
		}
		dto.ProgressMateri = strconv.Itoa(len(materiIDs)) + "/" + strconv.Itoa(int(totalMateri))

		if a, ok := aggMap[sw.ID]; ok && a.Total > 0 {
			dto.SoalDikerjakan = int(a.Total)
			dto.NilaiRata = float64(a.Benar) / float64(a.Total) * 100
			sumNilai += dto.NilaiRata
			countNilai++
		}

		switch {
		case dto.SoalDikerjakan > 0 && dto.NilaiRata < 60:
			dto.Status = "perlu_perhatian"
			ring.PerluPerhatian++
		case dto.SoalDikerjakan == 0 && dto.MateriSelesai == 0:
			dto.Status = "belum_aktif"
		default:
			dto.Status = "aman"
		}

		ring.MateriSelesai += dto.MateriSelesai
		out = append(out, dto)
	}
	if countNilai > 0 {
		ring.RataNilai = sumNilai / float64(countNilai)
	}
	return ring, out
}