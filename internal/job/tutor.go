package job

import (
	"encoding/json"
	"fmt"
)

// TutorResult adalah hasil callback tutor yang terstruktur dari AI Service.
type TutorResult struct {
	Balasan     string // teks untuk TTS
	Fase        string // "onboarding" | "belajar" | "" (kosong = AI belum mengisi)
	ExtractNama string // nama siswa (jika terkumpul saat onboarding)
	ExtractKode string // kode kelas 6 digit (jika sudah dikonfirmasi)
}

// ParseTutorFull ekstrak hasil tutor lengkap (balasan + fase + extract).
// Mendukung bentuk datar {"balasan":...} maupun wrapped {"data":{"balasan":...}}.
func ParseTutorFull(raw json.RawMessage) (*TutorResult, error) {
	// Parse ke map dulu biar 100% aman dari struktur JSON yang berubah-ubah atau field kosong
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("gagal unmarshal hasil tutor: %w", err)
	}

	// 1. Ambil balasan (kasih fallback kalau AI ngirim string kosong)
	balasan, _ := payload["balasan"].(string)
	if balasan == "" {
		balasan = "Maaf, aku kurang jelas mendengar. Bisa diulangi?"
	}

	// 2. Ambil fase
	fase, _ := payload["fase"].(string)
	if fase == "" {
		fase = "onboarding"
	}

	// 3. Ambil extract (nama & kode_kelas)
	var extractNama, extractKode string
	if ext, ok := payload["extract"].(map[string]interface{}); ok {
		extractNama, _ = ext["nama"].(string)
		extractKode, _ = ext["kode_kelas"].(string)
	}

	return &TutorResult{
		Balasan:     balasan,
		Fase:        fase,
		ExtractNama: extractNama,
		ExtractKode: extractKode,
	}, nil
}

// ParseTutorResult ekstrak hanya teks balasan (kompatibel lama, dipakai callback handler).
func ParseTutorResult(raw json.RawMessage) (string, error) {
	res, err := ParseTutorFull(raw)
	if err != nil {
		return "", err
	}
	return res.Balasan, nil
}