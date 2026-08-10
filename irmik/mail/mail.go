// Package mail defines a small email Sender interface with a net/smtp implementation.
//
// Opt-in: import only when sending mail. No SMTP client is started by irmik.New.
package mail

import (
	"context"
	"fmt"
	"net"
	"net/smtp"
	"strings"
)

// Message is a simple outbound email.
type Message struct {
	From    string
	To      []string
	Subject string
	// Body is plain text by default.
	Body string
	// HTML, when set, is sent as multipart/alternative alongside Body.
	HTML string
	// Headers are extra RFC 5322 headers (e.g. "Reply-To").
	Headers map[string]string
}

// Sender sends email.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// SMTPConfig configures net/smtp delivery.
type SMTPConfig struct {
	Host     string // e.g. smtp.example.com
	Port     int    // default 587
	Username string
	Password string
	// From is the default envelope/from when Message.From is empty.
	From string
	// InsecureSkipAuth sends without AUTH (local relay / MailHog).
	InsecureSkipAuth bool
}

// SMTP is a Sender backed by net/smtp.
type SMTP struct {
	cfg SMTPConfig
}

// NewSMTP returns an SMTP sender.
func NewSMTP(cfg SMTPConfig) *SMTP {
	if cfg.Port == 0 {
		cfg.Port = 587
	}
	return &SMTP{cfg: cfg}
}

// Send delivers msg via SMTP. ctx cancellation is checked before dial.
func (s *SMTP) Send(ctx context.Context, msg Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(msg.To) == 0 {
		return fmt.Errorf("mail: no recipients")
	}
	from := msg.From
	if from == "" {
		from = s.cfg.From
	}
	if from == "" {
		return fmt.Errorf("mail: from address required")
	}
	addr := net.JoinHostPort(s.cfg.Host, fmt.Sprintf("%d", s.cfg.Port))
	raw, err := buildMessage(from, msg)
	if err != nil {
		return err
	}
	var auth smtp.Auth
	if !s.cfg.InsecureSkipAuth && s.cfg.Username != "" {
		auth = smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	}
	return smtp.SendMail(addr, auth, from, msg.To, raw)
}

func buildMessage(from string, msg Message) ([]byte, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", strings.Join(msg.To, ", "))
	fmt.Fprintf(&b, "Subject: %s\r\n", sanitizeHeader(msg.Subject))
	for k, v := range msg.Headers {
		fmt.Fprintf(&b, "%s: %s\r\n", sanitizeHeader(k), sanitizeHeader(v))
	}
	fmt.Fprintf(&b, "MIME-Version: 1.0\r\n")
	if msg.HTML != "" {
		boundary := "irmik-mail-boundary"
		fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=%s\r\n\r\n", boundary)
		fmt.Fprintf(&b, "--%s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n", boundary, msg.Body)
		fmt.Fprintf(&b, "--%s\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s\r\n", boundary, msg.HTML)
		fmt.Fprintf(&b, "--%s--\r\n", boundary)
	} else {
		fmt.Fprintf(&b, "Content-Type: text/plain; charset=UTF-8\r\n\r\n%s", msg.Body)
	}
	return []byte(b.String()), nil
}

func sanitizeHeader(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r", ""), "\n", "")
}

// Memory is an in-process Sender for tests.
type Memory struct {
	Messages []Message
}

// Send appends msg to Messages.
func (m *Memory) Send(ctx context.Context, msg Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.Messages = append(m.Messages, msg)
	return nil
}
