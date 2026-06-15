package outbound

import (
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
)

type FactoryConfig struct {
	APIBaseURL string
	BotToken   string
	Timeout    time.Duration
}

func NewSharedGateway(o11y observability.Observability, cfg FactoryConfig) (*Gateway, error) {
	if cfg.APIBaseURL == "" {
		return nil, fmt.Errorf("telegram.outbound.factory: api_base_url is required")
	}
	if cfg.BotToken == "" {
		return nil, fmt.Errorf("telegram.outbound.factory: bot_token is required")
	}
	client, err := httpclient.NewClient(o11y,
		httpclient.WithBaseURL(cfg.APIBaseURL),
		httpclient.WithTarget("telegram_bot_api"),
		httpclient.WithTimeout(cfg.Timeout),
	)
	if err != nil {
		return nil, fmt.Errorf("telegram.outbound.factory: http client: %w", err)
	}
	return NewGateway(client, cfg.BotToken, o11y), nil
}
