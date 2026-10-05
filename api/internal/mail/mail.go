// Package mail envia e-mails de convite/recuperação. Em dev (MAIL_DEV)
// o destino é o MailPit; sem SMTP configurado, o link vai para o Log
// (fallback operacional — nunca na resposta HTTP, ARQUITETURA §5.3/§4.5).
package mail

import (
	"context"
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"
	"time"

	"github.com/seno-project/seno/api/internal/platform"
)

// Message é um e-mail plano do sistema.
type Message struct {
	To      string
	Subject string
	Text    string
}

// Sender é a forma mínima de despacho de e-mail.
type Sender interface {
	Send(ctx context.Context, m Message) error
}

// SMTPSender envia via SMTP real (ou MailPit no dev).
type SMTPSender struct {
	Host, User, Password, From string
	Port                       int
}

// Send implementa Sender via net/smtp com AUTH PLAIN opcional.
func (s *SMTPSender) Send(_ context.Context, m Message) error {
	addr := fmt.Sprintf("%s:%d", s.Host, s.Port)
	headers := map[string]string{
		"From":         s.From,
		"To":           m.To,
		"Subject":      m.Subject,
		"MIME-Version": "1.0",
		"Content-Type": "text/plain; charset=UTF-8",
	}
	var b strings.Builder
	for k, v := range headers {
		fmt.Fprintf(&b, "%s: %s\r\n", k, v)
	}
	b.WriteString("\r\n")
	b.WriteString(m.Text)

	var auth smtp.Auth
	if s.User != "" {
		auth = smtp.PlainAuth("", s.User, s.Password, s.Host)
	}
	// O timeout é tratado pelo caller (contexto não flui para net/smtp).
	return smtp.SendMail(addr, auth, s.From, []string{m.To}, []byte(b.String()))
}

// LogSender registra o conteúdo no Log de operação. É também o fallback
// quando o SMTP falha (§5.3).
type LogSender struct {
	Audit *platform.Audit
}

// Send implementa Sender gravando o e-mail no Log.
func (l *LogSender) Send(ctx context.Context, m Message) error {
	l.Audit.Record(ctx, platform.LogEntry{
		Actor:  "system",
		Kind:   "mail.fallback",
		Detail: fmt.Sprintf("para=%s assunto=%q texto=%q", m.To, m.Subject, m.Text),
	})
	slog.Warn("SMTP indisponível: e-mail enviado para o Log", "to", m.To)
	return nil
}

// Choose decide o canal: MAIL_DEV força MailPit; SMTP configurado usa SMTP;
// sem nada, Log.
func Choose(cfg *platform.Config, audit *platform.Audit) Sender {
	if cfg.SMTPHost != "" {
		return &SMTPSender{Host: cfg.SMTPHost, Port: cfg.SMTPPort, User: cfg.SMTPUser, Password: cfg.SMTPPassword, From: cfg.SMTPFrom}
	}
	if cfg.MailDev {
		return &SMTPSender{Host: "localhost", Port: 1025, From: cfg.SMTPFrom}
	}
	return &LogSender{Audit: audit}
}

// SendWithFallback tenta o canal principal e recorre ao Log em falha.
func SendWithFallback(ctx context.Context, primary Sender, fallback *LogSender, m Message, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := primary.Send(ctx, m); err != nil {
		slog.Error("falha no envio de e-mail; usando fallback em Log", "err", err, "to", m.To)
		return fallback.Send(ctx, m)
	}
	return nil
}
