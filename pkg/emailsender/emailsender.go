package emailsender

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type Client struct {
	apiKey     string
	httpClient *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}
}

type resendRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
}

func (c *Client) SendVerificationEmail(toEmail string, namaGuru string, verifyLink string) error {
	body := resendRequest{
		From:    "Momo <onboarding@resend.dev>",
		To:      []string{toEmail},
		Subject: "Verifikasi Email Akun Guru Momo",
		HTML: fmt.Sprintf(`
			<h2>Halo, %s!</h2>
			<p>Terima kasih sudah mendaftar sebagai Guru di aplikasi Momo.</p>
			<p>Klik tombol di bawah untuk memverifikasi email kamu:</p>
			<p><a href="%s" style="background:#4F46E5;color:white;padding:10px 20px;text-decoration:none;border-radius:5px;">Verifikasi Email</a></p>
			<p>Atau salin link berikut: %s</p>
		`, namaGuru, verifyLink, verifyLink),
	}

	jsonData, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("gagal encode request email: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.resend.com/emails", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("gagal membuat request email: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("gagal mengirim email: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Resend merespons dengan status %d", resp.StatusCode)
	}

	return nil
}