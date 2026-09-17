package job

import (
	"encoding/json"
	"fmt"

	"momo-be/pkg/aiclient"
)

// ParseProcessResult mengekstrak materi/soal dari hasil callback.
// Menangani 2 bentuk: datar {materi:[...], soal:[...]} atau dibungkus {success, data:{...}}
func ParseProcessResult(raw json.RawMessage) ([]aiclient.MateriItem, []aiclient.SoalItem, error) {
	if len(raw) == 0 {
		return nil, nil, fmt.Errorf("hasil callback kosong")
	}

	// Coba bentuk datar dulu
	var datar struct {
		Materi []aiclient.MateriItem `json:"materi"`
		Soal   []aiclient.SoalItem   `json:"soal"`
	}
	if err := json.Unmarshal(raw, &datar); err == nil && (len(datar.Materi) > 0 || len(datar.Soal) > 0) {
		return datar.Materi, datar.Soal, nil
	}

	// Coba bentuk dibungkus {success, data: {...}}
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

	return nil, nil, fmt.Errorf("bentuk hasil process tidak dikenali")
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

	return "", false, "", false, fmt.Errorf("bentuk hasil evaluate tidak dikenali")
}
