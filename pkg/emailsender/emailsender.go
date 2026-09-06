package emailsender

import (
	"fmt"
	"net/smtp"
)

type Client struct {
	host     string
	port     string
	username string
	password string
	fromName string
}

func NewClient(host, port, username, password, fromName string) *Client {
	return &Client{
		host:     host,
		port:     port,
		username: username,
		password: password,
		fromName: fromName,
	}
}

func (c *Client) SendVerificationEmail(toEmail string, namaGuru string, verifyLink string) error {
	subject := "Verifikasi Email Akun Guru Momo"
	body := fmt.Sprintf(`
		<h2>Halo, %s!</h2>
		<p>Terima kasih sudah mendaftar sebagai Guru di aplikasi Momo.</p>
		<p>Klik tombol di bawah untuk memverifikasi email kamu:</p>
		<p><a href="%s" style="background:#4F46E5;color:white;padding:10px 20px;text-decoration:none;border-radius:5px;">Verifikasi Email</a></p>
		<p>Atau salin link berikut: %s</p>
	`, namaGuru, verifyLink, verifyLink)

	msg := []byte(fmt.Sprintf(
		"From: %s <%s>\r\n"+
			"To: %s\r\n"+
			"Subject: %s\r\n"+
			"MIME-Version: 1.0\r\n"+
			"Content-Type: text/html; charset=UTF-8\r\n"+
			"\r\n"+
			"%s\r\n",
		c.fromName, c.username, toEmail, subject, body,
	))

	auth := smtp.PlainAuth("", c.username, c.password, c.host)
	addr := c.host + ":" + c.port

	err := smtp.SendMail(addr, auth, c.username, []string{toEmail}, msg)
	if err != nil {
		return fmt.Errorf("gagal mengirim email via SMTP: %w", err)
	}

	return nil
}