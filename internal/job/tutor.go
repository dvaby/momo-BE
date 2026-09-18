package job

import (
	"encoding/json"
	"fmt"
)

// ParseTutorResult ekstrak balasan tutor dari hasil callback.
// Support berbagai bentuk response AI Service (balasan/jawaban/response).
func ParseTutorResult(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", fmt.Errorf("hasil callback kosong")
	}

	// Bentuk 1: datar {balasan: "..."}
	var datar struct {
		Balasan string `json:"balasan"`
	}
	if err := json.Unmarshal(raw, &datar); err == nil && datar.Balasan != "" {
		return datar.Balasan, nil
	}

	// Bentuk 2: wrapped {data: {balasan: "..."}}
	var wrapped struct {
		Data struct {
			Balasan string `json:"balasan"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &wrapped); err == nil && wrapped.Data.Balasan != "" {
		return wrapped.Data.Balasan, nil
	}

	// Bentuk 3: alternatif field name (jawaban/response)
	var alt struct {
		Jawaban  string `json:"jawaban"`
		Response string `json:"response"`
	}
	if err := json.Unmarshal(raw, &alt); err == nil {
		if alt.Jawaban != "" {
			return alt.Jawaban, nil
		}
		if alt.Response != "" {
			return alt.Response, nil
		}
	}

	return "", fmt.Errorf("bentuk hasil tutor tidak dikenali: %s", string(raw))
}