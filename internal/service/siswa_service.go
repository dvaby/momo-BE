package service

import (
	"fmt"

	"momo-be/internal/model"
	"momo-be/internal/repository"
	"momo-be/pkg/jwtutil"
)

type SiswaService struct {
	repo       *repository.SiswaRepository
	kelasRepo  *repository.KelasRepository
	materiRepo *repository.MateriRepository
	soalRepo   *repository.SoalRepository
}

func NewSiswaService(repo *repository.SiswaRepository, kelasRepo *repository.KelasRepository, materiRepo *repository.MateriRepository, soalRepo *repository.SoalRepository) *SiswaService {
	return &SiswaService{repo: repo, kelasRepo: kelasRepo, materiRepo: materiRepo, soalRepo: soalRepo}
}

func (s *SiswaService) DaftarkanSiswa(kelasID, guruID uint, nama string) (*model.Siswa, error) {
	kelas, err := s.kelasRepo.FindByIDAndGuruID(kelasID, guruID)
	if err != nil || kelas == nil {
		return nil, fmt.Errorf("kelas tidak ditemukan atau Anda tidak memiliki akses ke kelas ini")
	}

	siswaSudahAda, err := s.repo.FindByNamaAndKelasID(nama, kelasID)
	if err == nil && siswaSudahAda != nil {
		return nil, fmt.Errorf("siswa dengan nama '%s' sudah terdaftar di kelas ini", nama)
	}

	siswa := &model.Siswa{
		KelasID: kelasID,
		Nama:    nama,
	}

	err = s.repo.Create(siswa)
	if err != nil {
		return nil, fmt.Errorf("gagal mendaftarkan siswa: %w", err)
	}

	return siswa, nil
}

func (s *SiswaService) JoinSiswa(kodeKelas string, nama string) (*model.Siswa, string, error) {
	kelas, err := s.kelasRepo.FindByKode(kodeKelas)
	if err != nil {
		return nil, "", fmt.Errorf("kelas dengan kode '%s' tidak ditemukan", kodeKelas)
	}

	// Cek apakah siswa dengan nama ini sudah terdaftar di kelas ini
	siswa, err := s.repo.FindByNamaAndKelasID(nama, kelas.ID)
	if err != nil || siswa == nil {
		// AUTO-REGISTER: Siswa belum terdaftar, buat otomatis
		siswa = &model.Siswa{
			KelasID: kelas.ID,
			Nama:    nama,
		}
		err = s.repo.Create(siswa)
		if err != nil {
			return nil, "", fmt.Errorf("gagal mendaftarkan siswa: %w", err)
		}
	}

	// Generate token untuk siswa
	token, err := jwtutil.GenerateToken(siswa.ID, kelas.ID)
	if err != nil {
		return nil, "", fmt.Errorf("gagal membuat token: %w", err)
	}

	return siswa, token, nil
}

// GetKelasSaya mengambil info kelas yang diikuti siswa + modul yang ditugaskan
func (s *SiswaService) GetKelasSaya(siswaID, kelasID uint) (map[string]interface{}, error) {
	kelas, err := s.kelasRepo.FindByID(kelasID)
	if err != nil {
		return nil, fmt.Errorf("kelas tidak ditemukan")
	}

	// Cek apakah siswa benar-benar anggota kelas ini
	siswa, err := s.repo.FindByID(siswaID)
	if err != nil || siswa.KelasID != kelasID {
		return nil, fmt.Errorf("akses ditolak: siswa ini bukan anggota kelas")
	}

	// Load modul yang ditugaskan ke kelas ini
	moduls := kelas.Modul

	// Untuk setiap modul, cek ketersediaan materi & jenis soal
	modulInfo := make([]map[string]interface{}, 0, len(moduls))
	for _, modul := range moduls {
		// Cek apakah ada materi
		materiList, _ := s.materiRepo.FindByModulID(modul.ID)
		punyaMateri := len(materiList) > 0

		// Cek jenis soal yang tersedia
		jenisSoalTersedia := make([]string, 0)
		for _, jenis := range []model.JenisSoal{model.JenisSoalHarian, model.JenisSoalUTS, model.JenisSoalUAS} {
			soalList, _ := s.soalRepo.FindByModulAndJenis(modul.ID, jenis)
			if len(soalList) > 0 {
				jenisSoalTersedia = append(jenisSoalTersedia, string(jenis))
			}
		}

		modulInfo = append(modulInfo, map[string]interface{}{
			"id":                  modul.ID,
			"nama":                modul.Nama,
			"deskripsi":           modul.Deskripsi,
			"punya_materi":        punyaMateri,
			"jenis_soal_tersedia": jenisSoalTersedia,
		})
	}

	return map[string]interface{}{
		"kelas": map[string]interface{}{
			"id":             kelas.ID,
			"nama_kelas":     kelas.NamaKelas,
			"mata_pelajaran": kelas.MataPelajaran,
			"kode_kelas":     kelas.KodeKelas,
		},
		"modul": modulInfo,
	}, nil
}

// GetMateriForSiswa mengambil list materi dalam modul (validasi modul tertaut ke kelas siswa)
func (s *SiswaService) GetMateriForSiswa(siswaID, kelasID, modulID uint) ([]model.Materi, error) {
	// Verifikasi siswa adalah anggota kelas
	siswa, err := s.repo.FindByID(siswaID)
	if err != nil || siswa.KelasID != kelasID {
		return nil, fmt.Errorf("akses ditolak: siswa ini bukan anggota kelas")
	}

	// Verifikasi modul tertaut ke kelas
	kelas, err := s.kelasRepo.FindByID(kelasID)
	if err != nil {
		return nil, fmt.Errorf("kelas tidak ditemukan")
	}

	modulFound := false
	for _, m := range kelas.Modul {
		if m.ID == modulID {
			modulFound = true
			break
		}
	}
	if !modulFound {
		return nil, fmt.Errorf("modul ini tidak ditugaskan untuk kelas Anda")
	}

	// Ambil materi
	materiList, err := s.materiRepo.FindByModulID(modulID)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil materi: %w", err)
	}

	return materiList, nil
}

// LinkSession mengikat session percakapan ke siswa (pengganti token untuk alur suara).
func (s *SiswaService) LinkSession(siswaID uint, sessionID string) error {
	return s.repo.SetSessionID(siswaID, sessionID)
}

// NamaKelasByID mengambil nama kelas untuk konfirmasi onboarding.
func (s *SiswaService) NamaKelasByID(kelasID uint) (string, error) {
	kelas, err := s.kelasRepo.FindByID(kelasID)
	if err != nil || kelas == nil {
		return "", fmt.Errorf("kelas tidak ditemukan")
	}
	return kelas.NamaKelas, nil
}
// KontenKelas berisi jumlah materi & soal yang tersedia di suatu kelas.
type KontenKelas struct {
	JumlahMateri int `json:"jumlah_materi"`
	JumlahSoal   int `json:"jumlah_soal"`
}


// HitungKontenKelas menghitung jumlah materi & soal di kelas via repository.
func (s *SiswaService) HitungKontenKelas(kelasID uint) KontenKelas {
	return KontenKelas{
		JumlahMateri: int(s.kelasRepo.HitungMateriKelas(kelasID)),
		JumlahSoal:   int(s.kelasRepo.HitungSoalKelas(kelasID)),
	}
}