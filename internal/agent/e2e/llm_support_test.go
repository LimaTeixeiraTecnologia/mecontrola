//go:build integration

package e2e_test

import (
	"os"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/require"

	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/providers/openrouter"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
)

func realParser(t *testing.T) *usecases.ParseInbound {
	t.Helper()
	baseURL := os.Getenv("OPENROUTER_BASE_URL")
	if baseURL == "" {
		baseURL = "https://openrouter.ai"
	}
	client, err := httpclient.NewClient(noop.NewProvider(),
		httpclient.WithBaseURL(baseURL),
		httpclient.WithTarget("openrouter_real"),
		httpclient.WithTimeout(30*time.Second),
	)
	require.NoError(t, err)
	slug := valueobjects.ModelSlugGeminiFlashLite()
	if override := os.Getenv("LLM_TEST_MODEL"); override != "" {
		parsed, perr := valueobjects.NewModelSlug(override)
		require.NoError(t, perr)
		slug = parsed
		t.Logf("usando modelo de teste: %s", slug.String())
	}
	provider := openrouter.NewProvider(client, openrouter.ProviderConfig{
		Slug:        slug,
		APIKey:      os.Getenv("OPENROUTER_API_KEY"),
		HTTPReferer: "https://mecontrola.app",
		XTitle:      "MeControla",
		MaxTokens:   256,
		Temperature: 0,
	}, noop.NewProvider())
	breaker := services.NewCircuitBreaker(services.CircuitBreakerConfig{MaxFailures: 5, FailureWindow: 30 * time.Second, OpenDuration: 60 * time.Second})
	chain, err := services.NewFallbackChain([]appservices.LLMProvider{provider}, breaker, noop.NewProvider())
	require.NoError(t, err)
	parser, err := usecases.NewParseInbound(chain, 2000, noop.NewProvider())
	require.NoError(t, err)
	return parser
}
