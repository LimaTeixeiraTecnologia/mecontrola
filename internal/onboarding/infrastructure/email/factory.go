package email

import (
	"fmt"
	"strings"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
)

type SenderFactory struct {
	cfg  configs.EmailConfig
	o11y observability.Observability
}

func NewSenderFactory(cfg configs.EmailConfig, o11y observability.Observability) *SenderFactory {
	return &SenderFactory{cfg: cfg, o11y: o11y}
}

func (f *SenderFactory) Build() (interfaces.EmailSender, error) {
	provider := strings.ToLower(strings.TrimSpace(f.cfg.Provider))
	switch provider {
	case "", "smtp":
		return NewSMTPSender(SMTPConfig{
			Host:     f.cfg.SMTPHost,
			Port:     f.cfg.SMTPPort,
			Username: f.cfg.SMTPUsername,
			Password: f.cfg.SMTPPassword,
			StartTLS: f.cfg.SMTPStartTLS,
			Timeout:  f.cfg.SMTPTimeout,
		}, f.o11y)
	case "resend":
		return NewResendSender(ResendConfig{
			APIKey:  f.cfg.ResendAPIKey,
			BaseURL: f.cfg.ResendBaseURL,
			Timeout: f.cfg.HTTPTimeout,
		}, f.o11y)
	default:
		return nil, fmt.Errorf("email: provider desconhecido: %q", provider)
	}
}
