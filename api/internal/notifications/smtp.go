package notifications

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/wneessen/go-mail"

	"github.com/malarscouterna/ms-utrustning/api/internal/crypto"
	"github.com/malarscouterna/ms-utrustning/api/internal/db"
)

// smtpConfig holds resolved SMTP settings for a single send.
type smtpConfig struct {
	from     string
	host     string
	port     int
	tlsMode  mail.TLSPolicy
	user     string
	password string
}

// SMTPNotifier sends email via SMTP. It resolves config per-send from msg.GroupID:
// per-group settings take precedence over system env vars.
type SMTPNotifier struct {
	Q *db.Queries
}

func (s *SMTPNotifier) Send(ctx context.Context, msg Message) error {
	cfg, err := s.resolveConfig(ctx, msg.GroupID)
	if err != nil {
		return fmt.Errorf("smtp: no config available: %w", err)
	}

	m := mail.NewMsg()
	if err := m.From(cfg.from); err != nil {
		return err
	}
	if err := m.To(msg.To); err != nil {
		return err
	}
	m.Subject(msg.Subject)
	m.SetBodyString(mail.TypeTextHTML, msg.Body)
	if msg.TextBody != "" {
		m.AddAlternativeString(mail.TypeTextPlain, msg.TextBody)
	}

	opts := []mail.Option{
		mail.WithPort(cfg.port),
		mail.WithTLSPolicy(cfg.tlsMode),
		mail.WithTimeout(10 * time.Second),
	}
	if cfg.user != "" {
		opts = append(opts,
			mail.WithSMTPAuth(mail.SMTPAuthPlain),
			mail.WithUsername(cfg.user),
			mail.WithPassword(cfg.password),
		)
	} else {
		opts = append(opts, mail.WithSMTPAuth(mail.SMTPAuthNoAuth))
	}
	c, err := mail.NewClient(cfg.host, opts...)
	if err != nil {
		return err
	}
	return c.DialAndSendWithContext(ctx, m)
}

func (s *SMTPNotifier) resolveConfig(ctx context.Context, groupID string) (smtpConfig, error) {
	gs, err := s.Q.GetGroupSettings(ctx, groupID)
	if err == nil && gs.SmtpHost != "" && len(gs.SmtpKeyEncrypted) > 0 {
		key, err := crypto.Decrypt(gs.SmtpKeyEncrypted)
		if err == nil {
			return smtpConfig{
				from:     gs.NotificationEmailFrom,
				host:     gs.SmtpHost,
				port:     int(gs.SmtpPort),
				tlsMode:  parseTLS(gs.SmtpTls),
				user:     gs.SmtpUser,
				password: string(key),
			}, nil
		}
	}

	// Fall back to system env vars.
	host := os.Getenv("SMTP_DEFAULT_HOST")
	if host == "" {
		return smtpConfig{}, fmt.Errorf("SMTP_DEFAULT_HOST not set")
	}
	port := 587
	if p, err := strconv.Atoi(os.Getenv("SMTP_DEFAULT_PORT")); err == nil {
		port = p
	}
	return smtpConfig{
		from:     os.Getenv("SMTP_DEFAULT_FROM"),
		host:     host,
		port:     port,
		tlsMode:  parseTLS(os.Getenv("SMTP_DEFAULT_TLS")),
		user:     os.Getenv("SMTP_DEFAULT_USER"),
		password: os.Getenv("SMTP_DEFAULT_KEY"),
	}, nil
}

func parseTLS(s string) mail.TLSPolicy {
	if s == "tls" {
		return mail.TLSMandatory
	}
	return mail.TLSOpportunistic // STARTTLS
}
