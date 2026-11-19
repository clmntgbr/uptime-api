package mail

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"
)

type SMTPConfig struct {
	Host string
	Port int
	From string
	// User/Password empty for Mailpit and other open relays.
	User     string
	Password string
}

type Sender struct {
	cfg SMTPConfig
}

func NewSender(cfg SMTPConfig) *Sender {
	if cfg.Port == 0 {
		cfg.Port = 1025
	}
	if cfg.From == "" {
		cfg.From = "noreply@uptime.local"
	}
	return &Sender{cfg: cfg}
}

func (s *Sender) Enabled() bool {
	return s != nil && s.cfg.Host != ""
}

const (
	smtpDialTimeout = 10 * time.Second
	// smtpSessionTimeout bounds the whole SMTP conversation. Without it, a silent
	// server blocks the consumer and freezes its Kafka partition.
	smtpSessionTimeout = 30 * time.Second
)

// Send delivers a plain HTML email via SMTP. Like smtp.SendMail it upgrades to
// STARTTLS when the server offers it, and authenticates only when User is set.
func (s *Sender) Send(to, subject, htmlBody string) error {
	if !s.Enabled() {
		return fmt.Errorf("smtp not configured")
	}
	to = strings.TrimSpace(to)
	if to == "" {
		return fmt.Errorf("empty recipient")
	}
	addr := net.JoinHostPort(s.cfg.Host, fmt.Sprintf("%d", s.cfg.Port))
	msg := buildMessage(s.cfg.From, to, subject, htmlBody)

	conn, err := (&net.Dialer{Timeout: smtpDialTimeout}).Dial("tcp", addr)
	if err != nil {
		return err
	}
	if err := conn.SetDeadline(time.Now().Add(smtpSessionTimeout)); err != nil {
		_ = conn.Close()
		return err
	}
	c, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		_ = conn.Close()
		return err
	}
	defer c.Close()

	if ok, _ := c.Extension("STARTTLS"); ok {
		if err := c.StartTLS(&tls.Config{ServerName: s.cfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	}
	if s.cfg.User != "" {
		if err := c.Auth(smtp.PlainAuth("", s.cfg.User, s.cfg.Password, s.cfg.Host)); err != nil {
			return err
		}
	}
	if err := c.Mail(s.cfg.From); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		_ = w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

func buildMessage(from, to, subject, htmlBody string) []byte {
	var b strings.Builder
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + to + "\r\n")
	b.WriteString("Subject: " + subject + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	b.WriteString("\r\n")
	b.WriteString(htmlBody)
	return []byte(b.String())
}
