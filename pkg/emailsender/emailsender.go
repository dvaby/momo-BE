package emailsender

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	apiKey      string
	senderEmail string
	senderName  string
}

func NewClient(apiKey, senderEmail, senderName string) *Client {
	return &Client{
		apiKey:      apiKey,
		senderEmail: senderEmail,
		senderName:  senderName,
	}
}

type brevoEmailRequest struct {
	Sender      map[string]string   `json:"sender"`
	To          []map[string]string `json:"to"`
	Subject     string              `json:"subject"`
	HTMLContent string              `json:"htmlContent"`
}

func (c *Client) SendVerificationEmail(toEmail string, namaGuru string, verifyLink string) error {
	url := "https://api.brevo.com/v3/smtp/email"

	htmlContent := fmt.Sprintf(`
		<h2>Halo, %s!</h2>
		<p>Terima kasih sudah mendaftar sebagai Guru di aplikasi Momo.</p>
		<p>Klik tombol di bawah untuk memverifikasi email kamu:</p>
		<p><a href="%s" style="background:#4F46E5;color:white;padding:10px 20px;text-decoration:none;border-radius:5px;">Verifikasi Email</a></p>
		<p>Atau salin link berikut: %s</p>
	`, namaGuru, verifyLink, verifyLink)

	reqBody := brevoEmailRequest{
		Sender: map[string]string{
			"name":  c.senderName,
			"email": c.senderEmail,
		},
		To: []map[string]string{
			{"email": toEmail},
		},
		Subject:     "Verifikasi Email Akun Guru Momo",
		HTMLContent: htmlContent,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("gagal memarshal payload email: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("gagal membuat request email: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", c.apiKey)

	// PENTING: Timeout 10 detik mencegah request menggantung selamanya jika jaringan bermasalah
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("gagal mengirim request ke Brevo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Brevo mengembalikan status error: %d", resp.StatusCode)
	}

	return nil
}