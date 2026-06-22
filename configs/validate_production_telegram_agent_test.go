package configs_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
)

func buildBaseConfig() *configs.Config {
	return &configs.Config{
		AppConfig: configs.AppConfig{Environment: "production"},
		DBConfig: configs.DBConfig{
			Host:            "db",
			Port:            5432,
			User:            "real_user",
			Password:        "a-strong-password-2026-prod-1",
			Name:            "mecontrola",
			MaxConns:        25,
			MinConns:        2,
			MaxIdleConns:    5,
			ConnMaxLifetime: 30 * time.Minute,
			ConnMaxIdleTime: 10 * time.Minute,
		},
		HTTPConfig: configs.HTTPConfig{
			ServiceNameAPI:     "api",
			Port:               8080,
			CORSAllowedOrigins: "https://app.example.com",
		},
		AuthRateLimit: configs.AuthRateLimitConfig{
			PerUserPerMin: 60,
			PerUserBurst:  10,
		},
		WhatsAppConfig: configs.WhatsAppConfig{
			WebhookRateLimitPerMin: 600,
			WebhookRateLimitBurst:  100,
		},
	}
}

func TestValidateProductionTelegram_DisabledSkips(t *testing.T) {
	cfg := buildBaseConfig()
	cfg.TelegramConfig = configs.TelegramConfig{Enabled: false}

	err := cfg.Validate()
	if err != nil {
		assert.NotContains(t, err.Error(), "TELEGRAM_")
	}
}

func TestValidateProductionTelegram_EnabledMissingFieldsRejects(t *testing.T) {
	cfg := buildBaseConfig()
	cfg.TelegramConfig = configs.TelegramConfig{Enabled: true}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation errors")
	}
	msg := err.Error()
	assert.Contains(t, msg, "TELEGRAM_BOT_TOKEN obrigatorio")
	assert.Contains(t, msg, "TELEGRAM_BOT_ID")
	assert.Contains(t, msg, "TELEGRAM_SECRET_TOKEN obrigatorio")
	assert.Contains(t, msg, "TELEGRAM_API_BASE_URL obrigatorio")
	assert.Contains(t, msg, "TELEGRAM_OUTBOUND_TIMEOUT")
}

func TestValidateProductionTelegram_AcceptableConfigPasses(t *testing.T) {
	cfg := buildBaseConfig()
	cfg.TelegramConfig = configs.TelegramConfig{
		Enabled:                true,
		BotToken:               "real-bot-token",
		BotID:                  12345,
		SecretToken:            "real-secret-token-1234567890",
		APIBaseURL:             "https://api.telegram.org",
		OutboundTimeout:        10 * time.Second,
		WebhookRateLimitPerMin: 600,
		WebhookRateLimitBurst:  100,
	}

	err := cfg.Validate()
	if err != nil {
		assert.NotContains(t, err.Error(), "TELEGRAM_")
	}
}

func TestValidateProductionTelegram_OutboundTimeoutOutOfRange(t *testing.T) {
	cfg := buildBaseConfig()
	cfg.TelegramConfig = configs.TelegramConfig{
		Enabled:         true,
		BotToken:        "tok",
		BotID:           1,
		SecretToken:     "sec",
		APIBaseURL:      "https://api.telegram.org",
		OutboundTimeout: 60 * time.Second,
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected timeout validation error")
	}
	assert.Contains(t, err.Error(), "TELEGRAM_OUTBOUND_TIMEOUT")
}

func TestValidateProductionAgent_OpenRouterMissingFields(t *testing.T) {
	cfg := buildBaseConfig()
	cfg.AgentConfig = configs.AgentConfig{}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation errors")
	}
	msg := err.Error()
	assert.Contains(t, msg, "OPENROUTER_API_KEY obrigatorio")
	assert.Contains(t, msg, "AGENT_LLM_PRIMARY_MODEL obrigatorio")
	assert.Contains(t, msg, "AGENT_LLM_MAX_TOKENS")
	assert.Contains(t, msg, "AGENT_LLM_REQUEST_TIMEOUT")
}

func TestValidateProductionAgent_AcceptableConfigPasses(t *testing.T) {
	cfg := buildBaseConfig()
	cfg.AgentConfig = configs.AgentConfig{
		OpenRouterAPIKey: "sk-real-key",
		PrimaryModel:     "google/gemini-2.5-flash-lite",
		MaxTokens:        256,
		MaxInputChars:    2000,
		RequestTimeout:   8 * time.Second,
	}

	err := cfg.Validate()
	if err != nil {
		assert.NotContains(t, err.Error(), "OPENROUTER_")
		assert.NotContains(t, err.Error(), "AGENT_LLM_")
	}
}

func TestValidateProductionAgent_MaxTokensOutOfRange(t *testing.T) {
	cfg := buildBaseConfig()
	cfg.AgentConfig = configs.AgentConfig{
		OpenRouterAPIKey: "sk-real-key",
		PrimaryModel:     "google/gemini-2.5-flash-lite",
		MaxTokens:        99999,
		RequestTimeout:   8 * time.Second,
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected max_tokens validation error")
	}
	assert.Contains(t, err.Error(), "AGENT_LLM_MAX_TOKENS")
}
