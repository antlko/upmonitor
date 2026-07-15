package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/smtp"
	"strings"
)

type emailConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`
	To       string `json:"to"`
}

type emailSender struct{}

func init() { register("email", emailSender{}) }

// Send delivers via SMTP with STARTTLS/plain auth (v1 targets the common port
// 587 pattern; implicit-TLS on port 465 is not supported). net/smtp has no
// context support, so ctx is currently only honoured up to the dial.
func (emailSender) Send(_ context.Context, raw json.RawMessage, msg Message) error {
	var cfg emailConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return err
	}
	if cfg.Host == "" || cfg.From == "" || cfg.To == "" {
		return errors.New("email: host, from and to are required")
	}
	port := cfg.Port
	if port == 0 {
		port = 587
	}
	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	}
	addr := fmt.Sprintf("%s:%d", cfg.Host, port)
	return smtp.SendMail(addr, auth, cfg.From, splitRecipients(cfg.To), buildMessage(cfg, msg))
}

// buildMessage renders a minimal RFC 822 text/plain email (pure; unit-tested).
func buildMessage(cfg emailConfig, msg Message) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", cfg.From)
	fmt.Fprintf(&b, "To: %s\r\n", cfg.To)
	fmt.Fprintf(&b, "Subject: %s\r\n", msg.Subject())
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
	b.WriteString(msg.Body())
	return []byte(b.String())
}

// splitRecipients splits a comma-separated recipient list, trimming blanks.
func splitRecipients(to string) []string {
	parts := strings.Split(to, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
