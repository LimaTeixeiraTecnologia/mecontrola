package e2e_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/providers/openrouter"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
)

func TestComposeConversationalReply_E2E_FullStackOverHTTP(t *testing.T) {
	var capturedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		capturedBody = string(raw)
		_, _ = w.Write([]byte(`{
			"choices":[{"message":{"role":"assistant","content":"Vamos organizar suas finanças juntos! 💪"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":900,"completion_tokens":15}
		}`))
	}))
	t.Cleanup(server.Close)

	client, err := httpclient.NewClient(noop.NewProvider(),
		httpclient.WithBaseURL(server.URL),
		httpclient.WithTarget("openrouter_e2e"),
		httpclient.WithTimeout(2*time.Second),
	)
	require.NoError(t, err)

	provider := openrouter.NewProvider(client, openrouter.ProviderConfig{
		Slug:        valueobjects.ModelSlugGeminiFlashLite(),
		APIKey:      "test-key",
		HTTPReferer: "https://mecontrola.app",
		XTitle:      "MeControla",
		MaxTokens:   256,
		Temperature: 0,
	}, noop.NewProvider())

	breaker := services.NewCircuitBreaker(services.CircuitBreakerConfig{
		MaxFailures:   5,
		FailureWindow: 30 * time.Second,
		OpenDuration:  60 * time.Second,
	})
	chain, err := services.NewFallbackChain([]interfaces.LLMProvider{provider}, breaker, noop.NewProvider())
	require.NoError(t, err)

	uc, err := usecases.NewComposeConversationalReply(chain, 150, noop.NewProvider(), nil, nil, nil, nil)
	require.NoError(t, err)

	out, err := uc.Execute(context.Background(), usecases.ComposeConversationalInput{
		UserID:  uuid.New(),
		Channel: "whatsapp",
		Text:    "me ajuda a organizar minha vida financeira",
	})

	require.NoError(t, err)
	assert.Equal(t, "Vamos organizar suas finanças juntos! 💪", out.Reply)
	assert.NotContains(t, capturedBody, "response_format")
	assert.Contains(t, capturedBody, `"max_tokens":150`)
	assert.Contains(t, capturedBody, "MeControla")
}

func TestComposeConversationalReply_E2E_ProviderDownReturnsRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"boom"}}`))
	}))
	t.Cleanup(server.Close)

	client, err := httpclient.NewClient(noop.NewProvider(),
		httpclient.WithBaseURL(server.URL),
		httpclient.WithTarget("openrouter_e2e"),
		httpclient.WithTimeout(2*time.Second),
	)
	require.NoError(t, err)

	provider := openrouter.NewProvider(client, openrouter.ProviderConfig{
		Slug:      valueobjects.ModelSlugGeminiFlashLite(),
		APIKey:    "test-key",
		MaxTokens: 256,
	}, noop.NewProvider())

	breaker := services.NewCircuitBreaker(services.CircuitBreakerConfig{MaxFailures: 5, FailureWindow: 30 * time.Second, OpenDuration: 60 * time.Second})
	chain, err := services.NewFallbackChain([]interfaces.LLMProvider{provider}, breaker, noop.NewProvider())
	require.NoError(t, err)

	uc, err := usecases.NewComposeConversationalReply(chain, 150, noop.NewProvider(), nil, nil, nil, nil)
	require.NoError(t, err)

	out, err := uc.Execute(context.Background(), usecases.ComposeConversationalInput{UserID: uuid.New(), Channel: "whatsapp", Text: "oi"})

	require.NoError(t, err)
	require.NotEmpty(t, out.Reply)
	assert.Contains(t, out.Reply, "finanças")
}
