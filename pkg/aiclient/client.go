package aiclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// ============================================================
// METHOD LAMA (tetap dipakai alur sync saat ini — TIDAK DIUBAH)
// ============================================================

func (c *Client) ProcessText(tipe string, teksMentah string) (*ProcessResponse, error) {
	reqBody := ProcessRequest{
		Tipe:       tipe,
		TeksMentah: teksMentah,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("gagal encode request: %w", err)
	}

	url := c.baseURL + "/process"
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("gagal menghubungi AI Service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AI Service merespons dengan status %d", resp.StatusCode)
	}

	var result ProcessResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("gagal decode response AI Service: %w", err)
	}

	return &result, nil
}

func (c *Client) EvaluateAnswer(req EvaluateRequest) (*EvaluateResponse, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("gagal encode request: %w", err)
	}

	url := c.baseURL + "/evaluate"
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("gagal menghubungi AI Service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AI Service merespons dengan status %d", resp.StatusCode)
	}

	var result EvaluateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("gagal decode response AI Service: %w", err)
	}

	return &result, nil
}

// ============================================================
// METHOD BARU — DUAL MODE untuk arsitektur v1.4+
//
// Mengirim job_id + callback_url. Dua kemungkinan response:
//   1. AI return {accepted: true, job_id: ...}  → path ASYNC (pola baru)
//   2. AI return hasil langsung                 → path SYNC (pola lama)
//
// Return: (ack, processResp, evalResp, err)
//   - Jika ack != nil → pola baru, tunggu callback di endpoint internal
//   - Jika processResp != nil → pola lama (materi/soal)
//   - Jika evalResp != nil → pola lama (evaluate)
// ============================================================

type DualModeResult struct {
	Ack           *JobAck           // tidak nil jika AI pakai pola baru (ack)
	ProcessResult *ProcessResponse  // tidak nil jika AI pola lama (tipe materi/soal)
	EvalResult    *EvaluateResponse // tidak nil jika AI pola lama (tipe evaluate)
}

// ProcessTextWithJob mengirim request /process dengan job_id + callback_url + konteks.
func (c *Client) ProcessTextWithJob(tipe, teksMentah, jobID, callbackURL, konteks string) (*DualModeResult, error) {
	reqBody := ProcessRequest{
		Tipe:        tipe,
		TeksMentah:  teksMentah,
		JobID:       jobID,
		CallbackURL: callbackURL,
		Konteks:     konteks,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("gagal encode request: %w", err)
	}

	url := c.baseURL + "/process"
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("gagal menghubungi AI Service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AI Service merespons dengan status %d", resp.StatusCode)
	}

	// Baca body sekali untuk deteksi pola
	var raw map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("gagal decode response AI Service: %w", err)
	}

	result := &DualModeResult{}

	// Deteksi pola baru: ada field "accepted" == true
	if accepted, ok := raw["accepted"].(bool); ok && accepted {
		ack := &JobAck{Accepted: true}
		if jid, ok := raw["job_id"].(string); ok {
			ack.JobID = jid
		}
		result.Ack = ack
		return result, nil
	}

	// Pola lama: reconstruct ke ProcessResponse
	// Marshal ulang lalu decode ke struct yang tepat
	bodyBytes, _ := json.Marshal(raw)
	var processResp ProcessResponse
	if err := json.Unmarshal(bodyBytes, &processResp); err == nil {
		result.ProcessResult = &processResp
		return result, nil
	}

	return nil, fmt.Errorf("response AI Service tidak dikenali (bukan ack maupun hasil)")
}

// EvaluateAnswerWithJob mengirim request /evaluate dengan job_id + callback_url + session_id + bootstrap + konteks.
func (c *Client) EvaluateAnswerWithJob(req EvaluateRequest) (*DualModeResult, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("gagal encode request: %w", err)
	}

	url := c.baseURL + "/evaluate"
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("gagal menghubungi AI Service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AI Service merespons dengan status %d", resp.StatusCode)
	}

	var raw map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("gagal decode response AI Service: %w", err)
	}

	result := &DualModeResult{}

	if accepted, ok := raw["accepted"].(bool); ok && accepted {
		ack := &JobAck{Accepted: true}
		if jid, ok := raw["job_id"].(string); ok {
			ack.JobID = jid
		}
		result.Ack = ack
		return result, nil
	}

	bodyBytes, _ := json.Marshal(raw)
	var evalResp EvaluateResponse
	if err := json.Unmarshal(bodyBytes, &evalResp); err == nil {
		result.EvalResult = &evalResp
		return result, nil
	}

	return nil, fmt.Errorf("response AI Service tidak dikenali")
}
