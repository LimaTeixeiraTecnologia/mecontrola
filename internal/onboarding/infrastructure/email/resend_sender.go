package email

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
)

type ResendConfig struct {
	APIKey  string
	BaseURL string
	Timeout time.Duration
}

type ResendSender struct {
	cfg    ResendConfig
	client *http.Client
	o11y   observability.Observability
	sent   observability.Counter
}

func NewResendSender(cfg ResendConfig, o11y observability.Observability) (*ResendSender, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, errors.New("email: resend: api key vazia")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.resend.com"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	sent := o11y.Metrics().Counter(
		"onboarding_email_sent_total",
		"Total de emails enviados via Resend",
		"1",
	)
	return &ResendSender{
		cfg:    cfg,
		client: &http.Client{Timeout: cfg.Timeout},
		o11y:   o11y,
		sent:   sent,
	}, nil
}

type resendRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html,omitempty"`
	Text    string   `json:"text,omitempty"`
	ReplyTo string   `json:"reply_to,omitempty"`
}

func (s *ResendSender) Send(ctx context.Context, msg interfaces.EmailMessage) error {
	ctx, span := s.o11y.Tracer().Start(ctx, "onboarding.email.resend.send")
	defer span.End()

	if strings.TrimSpace(msg.To) == "" {
		return errors.New("email: resend: destinatario vazio")
	}

	from := msg.FromAddress
	if msg.FromName != "" {
		from = fmt.Sprintf("%s <%s>", msg.FromName, msg.FromAddress)
	}

	body := resendRequest{
		From:    from,
		To:      []string{msg.To},
		Subject: msg.Subject,
		HTML:    msg.HTMLBody,
		Text:    msg.TextBody,
		ReplyTo: msg.ReplyTo,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("email: resend: encode payload: %w", err)
	}

	url := strings.TrimRight(s.cfg.BaseURL, "/") + "/emails"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("email: resend: criar request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		s.sent.Add(ctx, 1, observability.String("result", "transport_failed"))
		return fmt.Errorf("email: resend: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		s.sent.Add(ctx, 1, observability.String("result", fmt.Sprintf("http_%d", resp.StatusCode)))
		return fmt.Errorf("email: resend: status %d body=%s", resp.StatusCode, string(payload))
	}

	s.sent.Add(ctx, 1, observability.String("result", "ok"))
	return nil
}
