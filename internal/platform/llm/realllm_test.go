//go:build integration

package llm

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
)

func buildRealProvider(t *testing.T) Provider {
	t.Helper()
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" || os.Getenv("RUN_REAL_LLM") != "1" {
		t.Skip("RUN_REAL_LLM=1 and OPENROUTER_API_KEY required for real LLM tests")
	}

	baseURL := "https://openrouter.ai"
	client, err := httpclient.NewClient(fake.NewProvider(),
		httpclient.WithBaseURL(baseURL),
		httpclient.WithTarget("openrouter"),
		httpclient.WithTimeout(30*time.Second),
	)
	require.NoError(t, err)

	return NewOpenRouterProvider(client, Config{
		Model:          "google/gemini-2.5-flash-lite",
		EmbedModel:     "openai/text-embedding-3-small",
		BaseURL:        baseURL,
		APIKey:         apiKey,
		HTTPReferer:    "https://github.com/LimaTeixeiraTecnologia/mecontrola",
		XTitle:         "mecontrola-integration-test",
		MaxTokens:      256,
		Temperature:    0,
		RequestTimeout: 30 * time.Second,
	}, fake.NewProvider())
}

func TestRealLLM_Complete(t *testing.T) {
	provider := buildRealProvider(t)
	ctx := context.Background()

	resp, err := provider.Complete(ctx, Request{
		Messages: []Message{
			{Role: "system", Content: "You are a helpful assistant. Reply with a single word."},
			{Role: "user", Content: "Say: hello"},
		},
		FreeText:  true,
		MaxTokens: 10,
	})

	require.NoError(t, err)
	require.NotEmpty(t, resp.Content)
	require.Greater(t, resp.PromptTokens, 0)
}

func TestRealLLM_Stream(t *testing.T) {
	provider := buildRealProvider(t)
	ctx := context.Background()

	stream, err := provider.Stream(ctx, Request{
		Messages: []Message{
			{Role: "system", Content: "Reply with one sentence maximum."},
			{Role: "user", Content: "Count to three."},
		},
		FreeText:  true,
		MaxTokens: 50,
	})
	require.NoError(t, err)
	defer stream.Close() //nolint:errcheck

	var collected string
	for delta := range stream.Deltas() {
		collected += delta
	}
	require.NoError(t, stream.Err())
	require.NotEmpty(t, collected)
}

func TestRealLLM_Embed(t *testing.T) {
	provider := buildRealProvider(t)
	ctx := context.Background()

	result, err := provider.Embed(ctx, []string{"hello world", "foo bar"})

	require.NoError(t, err)
	require.Len(t, result, 2)
	require.Len(t, result[0], 1536)
	require.Len(t, result[1], 1536)
}
