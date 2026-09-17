package job

import (
	"encoding/json"
	"fmt"
	"log"

	"momo-be/pkg/aiclient"
)

// ParseProcessResult mengekstrak materi/soal dari hasil callback.
// Menangani berbagai bentuk response dari AI Service (v1.6 baru).
func ParseProcessResult(raw json.RawMessage) ([]aiclient.MateriItem, []aiclient.SoalItem, error) {
	if len(raw) == 0 {
		return nil, nil, fmt.Errorf("hasil callback kosong")
	}

	// DEBUG: log raw response (first 500 chars) kalau gagal parse
	debugRaw := string(raw)
	if len(debugRaw) > 500 {
		debugRaw = debugRaw[:500] + "..."
	}

	// Coba bentuk 1: datar {materi:[...], soal:[...]}
	var datar struct {
		Materi []aiclient.MateriItem `json:"materi"`
		Soal   []aiclient.SoalItem   `json:"soal"`
	}
	if err := json.Unmarshal(raw, &datar); err == nil && (len(datar.Materi) > 0 || len(datar.Soal) > 0) {
		return datar.Materi, datar.Soal, nil
	}

	// Coba bentuk 2: {success, data:{...}}
	var wrapped struct {
		Success bool `json:"success"`
		Data    struct {
			Materi []aiclient.MateriItem `json:"materi"`
			Soal   []aiclient.SoalItem   `json:"soal"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &wrapped); err == nil && (len(wrapped.Data.Materi) > 0 || len(wrapped.Data.Soal) > 0) {
		return wrapped.Data.Materi, wrapped.Data.Soal, nil
	}

	// Coba bentuk 3: AI Service v1.6 baru mungkin return {hasil: {materi/soal: [...]}}
	var withHasil struct {
		Hasil struct {
			Materi []aiclient.MateriItem `json:"materi"`
			Soal   []aiclient.SoalItem   `json:"soal"`
		} `json:"hasil"`
	}
	if err := json.Unmarshal(raw, &withHasil); err == nil && (len(withHasil.Hasil.Materi) > 0 || len(withHasil.Hasil.Soal) > 0) {
		return withHasil.Hasil.Materi, withHasil.Hasil.Soal, nil
	}

	// Coba bentuk 4: langsung array soal/materi
	var langsungSoal []aiclient.SoalItem
	if err := json.Unmarshal(raw, &langsungSoal); err == nil && len(langsungSoal) > 0 {
		return nil, langsungSoal, nil
	}

	var langsungMateri []aiclient.MateriItem
	if err := json.Unmarshal(raw, &langsungMateri); err == nil && len(langsungMateri) > 0 {
		return langsungMateri, nil, nil
	}

	// Gagal semua — log raw response untuk debug
	log.Printf("[parse] WARNING: bentuk hasil tidak dikenali, raw: %s", debugRaw)
	return nil, nil, fmt.Errorf("bentuk hasil process tidak dikenali (lihat log untuk raw response)")
}

// ParseEvaluateResult mengekstrak feedback dari hasil callback /evaluate.
func ParseEvaluateResult(raw json.RawMessage) (string, bool, string, bool, error) {
	if len(raw) == 0 {
		return "", false, "", false, fmt.Errorf("hasil callback kosong")
	}

	// Bentuk datar
	var datar struct {
		JawabanTerdeteksi string `json:"jawaban_terdeteksi"`
		Benar             bool   `json:"benar"`
		Feedback          string `json:"feedback"`
		PerluKlarifikasi  bool   `json:"perlu_klarifikasi"`
	}
	if err := json.Unmarshal(raw, &datar); err == nil && datar.Feedback != "" {
		return datar.JawabanTerdeteksi, datar.Benar, datar.Feedback, datar.PerluKlarifikasi, nil
	}

	// Bentuk dibungkus
	var wrapped struct {
		Success bool `json:"success"`
		Data    struct {
			JawabanTerdeteksi string `json:"jawaban_terdeteksi"`
			Benar             bool   `json:"benar"`
			Feedback          string `json:"feedback"`
			PerluKlarifikasi  bool   `json:"perlu_klarifikasi"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &wrapped); err == nil && wrapped.Data.Feedback != "" {
		return wrapped.Data.JawabanTerdeteksi, wrapped.Data.Benar, wrapped.Data.Feedback, wrapped.Data.PerluKlarifikasi, nil
	}

	debugRaw := string(raw)
	if len(debugRaw) > 500 {
		debugRaw = debugRaw[:500] + "..."
	}
	log.Printf("[parse-evaluate] WARNING: bentuk hasil tidak dikenali, raw: %s", debugRaw)
	return "", false, "", false, fmt.Errorf("bentuk hasil evaluate tidak dikenali")
}