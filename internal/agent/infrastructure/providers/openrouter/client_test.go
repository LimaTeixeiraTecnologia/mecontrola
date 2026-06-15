package openrouter_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/providers/openrouter"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
)

func buildProvider(t *testing.T, server *httptest.Server) *openrouter.Provider {
	t.Helper()
	client, err := httpclient.NewClient(noop.NewProvider(),
		httpclient.WithBaseURL(server.URL),
		httpclient.WithTarget("openrouter_test"),
		httpclient.WithTimeout(2*time.Second),
	)
	require.NoError(t, err)
	return openrouter.NewProvider(client, openrouter.ProviderConfig{
		Slug:        valueobjects.ModelSlugGeminiFlashLite(),
		APIKey:      "test-key",
		HTTPReferer: "https://example.com",
		XTitle:      "TestApp",
		MaxTokens:   256,
		Temperature: 0,
	}, noop.NewProvider())
}

func TestProvider_Interpret_HappyPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "https://example.com", r.Header.Get("HTTP-Referer"))
		assert.Equal(t, "TestApp", r.Header.Get("X-Title"))

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		assert.Contains(t, string(body), `"model":"google/gemini-2.5-flash-lite"`)
		assert.Contains(t, string(body), `"json_schema"`)
		assert.Contains(t, string(body), `"strict":true`)
		assert.Contains(t, string(body), `"mecontrola_intent"`)

		_, _ = w.Write([]byte(`{
			"choices":[{"message":{"role":"assistant","content":"{\"module\":\"cards\",\"action\":\"list\",\"response_hint\":\"ok\"}"}}],
			"usage":{"prompt_tokens":700,"completion_tokens":80}
		}`))
	}))
	t.Cleanup(server.Close)

	sut := buildProvider(t, server)
	resp, err := sut.Interpret(context.Background(), interfaces.LLMRequest{
		SystemPrompt: "system",
		UserMessage:  "user",
	})

	require.NoError(t, err)
	assert.True(t, resp.Provider.Equal(valueobjects.ModelSlugGeminiFlashLite()))
	assert.Contains(t, string(resp.RawJSON), `"cards"`)
	assert.Equal(t, 700, resp.PromptTokens)
	assert.Equal(t, 80, resp.CompletionTokens)
}

func TestProvider_Interpret_401Unauthorized_ReturnsUpstream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid key"}}`))
	}))
	t.Cleanup(server.Close)

	sut := buildProvider(t, server)
	_, err := sut.Interpret(context.Background(), interfaces.LLMRequest{SystemPrompt: "s", UserMessage: "u"})

	require.Error(t, err)
	assert.True(t, errors.Is(err, openrouter.ErrProviderUpstream))
	assert.Contains(t, err.Error(), "401")
}

func TestProvider_Interpret_429RateLimited_ReturnsUpstream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"rate"}}`))
	}))
	t.Cleanup(server.Close)

	sut := buildProvider(t, server)
	_, err := sut.Interpret(context.Background(), interfaces.LLMRequest{SystemPrompt: "s", UserMessage: "u"})

	require.Error(t, err)
	assert.True(t, errors.Is(err, openrouter.ErrProviderUpstream))
}

func TestProvider_Interpret_5xx_ReturnsUpstream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":{"message":"upstream broken"}}`))
	}))
	t.Cleanup(server.Close)

	sut := buildProvider(t, server)
	_, err := sut.Interpret(context.Background(), interfaces.LLMRequest{SystemPrompt: "s", UserMessage: "u"})

	require.Error(t, err)
	assert.True(t, errors.Is(err, openrouter.ErrProviderUpstream))
}

func TestProvider_Interpret_EmptyChoices_Rejects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[],"usage":{}}`))
	}))
	t.Cleanup(server.Close)

	sut := buildProvider(t, server)
	_, err := sut.Interpret(context.Background(), interfaces.LLMRequest{SystemPrompt: "s", UserMessage: "u"})

	require.Error(t, err)
	assert.True(t, errors.Is(err, openrouter.ErrEmptyChoices))
}

func TestProvider_Interpret_DecodeError_Rejects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{not json`))
	}))
	t.Cleanup(server.Close)

	sut := buildProvider(t, server)
	_, err := sut.Interpret(context.Background(), interfaces.LLMRequest{SystemPrompt: "s", UserMessage: "u"})

	require.Error(t, err)
}

func TestProvider_Interpret_UpstreamErrorPayload_Rejects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"error":{"message":"safety filter triggered"}}`))
	}))
	t.Cleanup(server.Close)

	sut := buildProvider(t, server)
	_, err := sut.Interpret(context.Background(), interfaces.LLMRequest{SystemPrompt: "s", UserMessage: "u"})

	require.Error(t, err)
	assert.True(t, errors.Is(err, openrouter.ErrProviderUpstream))
	assert.Contains(t, err.Error(), "safety filter")
}

func TestProvider_Slug(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Cleanup(server.Close)

	sut := buildProvider(t, server)
	assert.True(t, sut.Slug().Equal(valueobjects.ModelSlugGeminiFlashLite()))
}

func TestProvider_Interpret_RespectsContextCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{}"}}],"usage":{}}`))
	}))
	t.Cleanup(server.Close)

	sut := buildProvider(t, server)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err := sut.Interpret(ctx, interfaces.LLMRequest{SystemPrompt: "s", UserMessage: "u"})
	require.Error(t, err)
}

func TestProvider_RequestBody_ContainsSystemAndUser(t *testing.T) {
	var capturedBody json.RawMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{}"}}],"usage":{}}`))
	}))
	t.Cleanup(server.Close)

	sut := buildProvider(t, server)
	_, err := sut.Interpret(context.Background(), interfaces.LLMRequest{
		SystemPrompt: "system-prompt-content",
		UserMessage:  "user-message-content",
	})
	require.NoError(t, err)

	s := string(capturedBody)
	assert.True(t, strings.Contains(s, `"role":"system"`))
	assert.True(t, strings.Contains(s, `"role":"user"`))
	assert.True(t, strings.Contains(s, "system-prompt-content"))
	assert.True(t, strings.Contains(s, "user-message-content"))
}
