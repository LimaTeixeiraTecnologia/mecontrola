package email

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/smtp"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
)

type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	StartTLS bool
	Timeout  time.Duration
}

type SMTPSender struct {
	cfg  SMTPConfig
	o11y observability.Observability
	sent observability.Counter
}

func NewSMTPSender(cfg SMTPConfig, o11y observability.Observability) (*SMTPSender, error) {
	if cfg.Host == "" {
		return nil, errors.New("email: smtp: host vazio")
	}
	if cfg.Port == 0 {
		return nil, errors.New("email: smtp: porta zero")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	sent := o11y.Metrics().Counter(
		"onboarding_email_sent_total",
		"Total de emails enviados via SMTP",
		"1",
	)
	return &SMTPSender{cfg: cfg, o11y: o11y, sent: sent}, nil
}

func (s *SMTPSender) Send(ctx context.Context, msg interfaces.EmailMessage) (err error) {
	ctx, span := s.o11y.Tracer().Start(ctx, "onboarding.email.smtp.send")
	defer span.End()

	if err := s.validateMessage(msg); err != nil {
		return err
	}

	client, err := s.newClient(ctx)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, s.closeClient(client, err == nil))
	}()

	if err := s.configureClient(ctx, client); err != nil {
		return err
	}
	if err := s.sendMessage(ctx, client, msg); err != nil {
		return err
	}

	s.sent.Add(ctx, 1, observability.String("result", "ok"))
	return nil
}

func (s *SMTPSender) validateMessage(msg interfaces.EmailMessage) error {
	if strings.TrimSpace(msg.To) == "" {
		return errors.New("email: smtp: destinatario vazio")
	}
	if strings.TrimSpace(msg.FromAddress) == "" {
		return errors.New("email: smtp: remetente vazio")
	}
	return nil
}

func (s *SMTPSender) newClient(ctx context.Context) (*smtp.Client, error) {
	addr := net.JoinHostPort(s.cfg.Host, fmt.Sprintf("%d", s.cfg.Port))
	dialer := &net.Dialer{Timeout: s.cfg.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		s.sent.Add(ctx, 1, observability.String("result", "dial_failed"))
		return nil, fmt.Errorf("email: smtp: dial %s: %w", addr, err)
	}
	client, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		s.sent.Add(ctx, 1, observability.String("result", "client_failed"))
		return nil, errors.Join(
			fmt.Errorf("email: smtp: novo cliente: %w", err),
			s.closeConn(conn),
		)
	}
	return client, nil
}

func (s *SMTPSender) configureClient(ctx context.Context, client *smtp.Client) error {
	if s.cfg.StartTLS {
		if err := client.StartTLS(&tls.Config{ServerName: s.cfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
			s.sent.Add(ctx, 1, observability.String("result", "starttls_failed"))
			return fmt.Errorf("email: smtp: starttls: %w", err)
		}
	}
	if s.cfg.Username != "" {
		auth := smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
		if err := client.Auth(auth); err != nil {
			s.sent.Add(ctx, 1, observability.String("result", "auth_failed"))
			return fmt.Errorf("email: smtp: auth: %w", err)
		}
	}
	return nil
}

func (s *SMTPSender) sendMessage(ctx context.Context, client *smtp.Client, msg interfaces.EmailMessage) error {
	if err := client.Mail(msg.FromAddress); err != nil {
		s.sent.Add(ctx, 1, observability.String("result", "mail_from_failed"))
		return fmt.Errorf("email: smtp: mail from: %w", err)
	}
	if err := client.Rcpt(msg.To); err != nil {
		s.sent.Add(ctx, 1, observability.String("result", "rcpt_failed"))
		return fmt.Errorf("email: smtp: rcpt: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		s.sent.Add(ctx, 1, observability.String("result", "data_failed"))
		return fmt.Errorf("email: smtp: data: %w", err)
	}
	if err := s.writePayload(ctx, writer, buildMimeMessage(msg)); err != nil {
		s.sent.Add(ctx, 1, observability.String("result", "write_failed"))
		return err
	}
	return nil
}

func (s *SMTPSender) writePayload(ctx context.Context, writer io.WriteCloser, payload string) error {
	if _, err := writer.Write([]byte(payload)); err != nil {
		return errors.Join(
			fmt.Errorf("email: smtp: write payload: %w", err),
			s.closeWriter(writer),
		)
	}
	if err := s.closeWriter(writer); err != nil {
		s.sent.Add(ctx, 1, observability.String("result", "close_failed"))
		return err
	}
	return nil
}

func (s *SMTPSender) closeClient(client *smtp.Client, graceful bool) error {
	if graceful {
		if err := client.Quit(); err != nil {
			return errors.Join(
				fmt.Errorf("email: smtp: quit client: %w", err),
				s.closeSMTPClient(client),
			)
		}
		return nil
	}
	return s.closeSMTPClient(client)
}

func (s *SMTPSender) closeSMTPClient(client *smtp.Client) error {
	if err := client.Close(); err != nil {
		return fmt.Errorf("email: smtp: close client: %w", err)
	}
	return nil
}

func (s *SMTPSender) closeConn(conn net.Conn) error {
	if err := conn.Close(); err != nil {
		return fmt.Errorf("email: smtp: close conn: %w", err)
	}
	return nil
}

func (s *SMTPSender) closeWriter(writer io.WriteCloser) error {
	if err := writer.Close(); err != nil {
		return fmt.Errorf("email: smtp: close data: %w", err)
	}
	return nil
}

func buildMimeMessage(msg interfaces.EmailMessage) string {
	from := msg.FromAddress
	if msg.FromName != "" {
		from = fmt.Sprintf("%s <%s>", msg.FromName, msg.FromAddress)
	}

	headers := []string{
		fmt.Sprintf("From: %s", from),
		fmt.Sprintf("To: %s", msg.To),
		fmt.Sprintf("Subject: %s", msg.Subject),
		"MIME-Version: 1.0",
	}
	if msg.ReplyTo != "" {
		headers = append(headers, fmt.Sprintf("Reply-To: %s", msg.ReplyTo))
	}

	boundary := "mecontrola-boundary-7c9f"
	headers = append(headers,
		fmt.Sprintf("Content-Type: multipart/alternative; boundary=%q", boundary),
	)

	var body strings.Builder
	body.WriteString(strings.Join(headers, "\r\n"))
	body.WriteString("\r\n\r\n")

	fmt.Fprintf(&body, "--%s\r\n", boundary)
	body.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
	body.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	body.WriteString(msg.TextBody)
	body.WriteString("\r\n\r\n")

	fmt.Fprintf(&body, "--%s\r\n", boundary)
	body.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	body.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	body.WriteString(msg.HTMLBody)
	body.WriteString("\r\n\r\n")

	fmt.Fprintf(&body, "--%s--\r\n", boundary)

	return body.String()
}
