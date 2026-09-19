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
	if len(raw) == 0 {
		return nil, fmt.Errorf("hasil callback kosong")
	}

	var node map[string]json.RawMessage
	if err := json.Unmarshal(raw, &node); err != nil {
		return nil, fmt.Errorf("hasil callback bukan JSON object: %w", err)
	}

	// Kalau wrapped {data:{...}} dan inner punya balasan, pakai inner
	if inner, ok := node["data"]; ok {
		var innerMap map[string]json.RawMessage
		if err := json.Unmarshal(inner, &innerMap); err == nil {
			if _, has := innerMap["balasan"]; has {
				node = innerMap
			}
		}
	}

	out := &TutorResult{}
	for _, key := range []string{"balasan", "jawaban", "response"} {
		if v, ok := node[key]; ok {
			var s string
			if json.Unmarshal(v, &s) == nil && s != "" {
				out.Balasan = s
				break
			}
		}
	}
	if out.Balasan == "" {
		return nil, fmt.Errorf("bentuk hasil tutor tidak dikenali: %s", string(raw))
	}

	if v, ok := node["fase"]; ok {
		var s string
		if json.Unmarshal(v, &s) == nil {
			out.Fase = s
		}
	}
	if v, ok := node["extract"]; ok {
		var ex struct {
			Nama      string `json:"nama"`
			KodeKelas string `json:"kode_kelas"`
		}
		if json.Unmarshal(v, &ex) == nil {
			out.ExtractNama = ex.Nama
			out.ExtractKode = ex.KodeKelas
		}
	}
	return out, nil
}

// ParseTutorResult ekstrak hanya teks balasan (kompatibel lama, dipakai callback handler).
func ParseTutorResult(raw json.RawMessage) (string, error) {
	res, err := ParseTutorFull(raw)
	if err != nil {
		return "", err
	}
	return res.Balasan, nil
}