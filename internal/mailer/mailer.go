// Package mailer abstracts outbound email so the auth and notification
// layers don't need to know whether SMTP is configured. In local dev
// (no SMTP_HOST set) it falls back to logging the email instead of
// silently failing or requiring a mail server.
package mailer

import (
	"fmt"
	"net/smtp"

	"github.com/lakhans7/eventbasedhttp/internal/logger"
)

type Mailer interface {
	Send(to, subject, body string) error
}

type SMTPMailer struct {
	Host, Port, User, Pass, From string
}

func (m *SMTPMailer) Send(to, subject, body string) error {
	addr := fmt.Sprintf("%s:%s", m.Host, m.Port)
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		m.From, to, subject, body)

	var auth smtp.Auth
	if m.User != "" {
		auth = smtp.PlainAuth("", m.User, m.Pass, m.Host)
	}
	return smtp.SendMail(addr, auth, m.From, []string{to}, []byte(msg))
}

// ConsoleMailer logs the email instead of sending it. Used for local
// development and tests when no SMTP credentials are configured.
type ConsoleMailer struct{}

func (ConsoleMailer) Send(to, subject, body string) error {
	logger.Get().Info().
		Str("to", to).
		Str("subject", subject).
		Msg("mailer: SMTP not configured, logging email instead of sending")
	return nil
}

func New(host, port, user, pass, from string) Mailer {
	if host == "" {
		return ConsoleMailer{}
	}
	return &SMTPMailer{Host: host, Port: port, User: user, Pass: pass, From: from}
}
