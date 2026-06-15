package outbound_test

import (
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/telegram/outbound"
)

func TestNewSharedGateway_ValidConfig(t *testing.T) {
	gateway, err := outbound.NewSharedGateway(noop.NewProvider(), outbound.FactoryConfig{
		APIBaseURL: "https://api.telegram.org",
		BotToken:   "test-bot-token",
		Timeout:    10 * time.Second,
	})
	require.NoError(t, err)
	assert.NotNil(t, gateway)
}

func TestNewSharedGateway_MissingAPIBaseURL(t *testing.T) {
	_, err := outbound.NewSharedGateway(noop.NewProvider(), outbound.FactoryConfig{
		BotToken: "test-bot-token",
		Timeout:  10 * time.Second,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "api_base_url is required")
}

func TestNewSharedGateway_MissingBotToken(t *testing.T) {
	_, err := outbound.NewSharedGateway(noop.NewProvider(), outbound.FactoryConfig{
		APIBaseURL: "https://api.telegram.org",
		Timeout:    10 * time.Second,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bot_token is required")
}

func TestNewSharedGateway_InvalidURL(t *testing.T) {
	_, err := outbound.NewSharedGateway(noop.NewProvider(), outbound.FactoryConfig{
		APIBaseURL: "://invalid",
		BotToken:   "tok",
		Timeout:    10 * time.Second,
	})
	require.Error(t, err)
}
