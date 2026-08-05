//go:build integration

package llm

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
)

func buildRealTranscriber(t *testing.T) Transcriber {
	t.Helper()
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" || os.Getenv("RUN_REAL_STT") != "1" {
		t.Skip("RUN_REAL_STT=1 and OPENROUTER_API_KEY required for real STT tests")
	}

	baseURL := os.Getenv("OPENROUTER_BASE_URL")
	if baseURL == "" {
		baseURL = "https://openrouter.ai"
	}
	client, err := httpclient.NewClient(fake.NewProvider(),
		httpclient.WithBaseURL(baseURL),
		httpclient.WithTarget("openrouter_stt"),
		httpclient.WithTimeout(30*time.Second),
	)
	require.NoError(t, err)

	return NewOpenRouterTranscriber(client, Config{
		BaseURL:                        baseURL,
		APIKey:                         apiKey,
		HTTPReferer:                    "https://github.com/LimaTeixeiraTecnologia/mecontrola",
		XTitle:                         "mecontrola-integration-test",
		STTTimeout:                     20 * time.Second,
		STTPreflightRateMicrousdPerSec: 34,
	}, fake.NewProvider())
}

func realAudioFormatFromPath(t *testing.T, path string) AudioFormat {
	t.Helper()
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ogg", ".opus":
		return AudioFormatOGG
	case ".m4a", ".aac", ".mp4":
		return AudioFormatM4A
	default:
		t.Fatalf("STT_REAL_AUDIO_FIXTURE com extensao nao suportada: %s", path)
		return ""
	}
}

func TestRealSTT_Transcribe(t *testing.T) {
	transcriber := buildRealTranscriber(t)
	ctx := context.Background()

	audioPath := os.Getenv("STT_REAL_AUDIO_FIXTURE")
	if audioPath == "" {
		t.Fatal("RUN_REAL_STT=1 exige STT_REAL_AUDIO_FIXTURE apontando para audio PT-BR real; o gate real nao pode ser considerado atendido sem fixture")
	}
	audio, err := os.ReadFile(audioPath)
	require.NoError(t, err)

	model := os.Getenv("STT_REAL_MODEL")
	if model == "" {
		model = "openai/whisper-large-v3"
	}

	resp, err := transcriber.Transcribe(ctx, TranscriptionRequest{
		Model:           model,
		Audio:           audio,
		Format:          realAudioFormatFromPath(t, audioPath),
		Language:        "pt",
		Temperature:     0,
		DurationMs:      1000,
		MaxCostMicrousd: 2000,
	})

	require.NoError(t, err)

	t.Logf("STT real: provider=%q language=%q truncated=%v cost_microusd=%v seconds=%v text_len=%d",
		resp.Provider, resp.Language, resp.Truncated, resp.CostMicrousd, resp.Usage.Seconds, len(resp.Text))

	require.Equal(t, "openrouter", resp.Provider)
	require.NotEmpty(t, strings.TrimSpace(resp.Text), "provider deve retornar texto para audio PT-BR real")
	require.NotEmpty(t, resp.Language, "provider deve retornar o campo language; o gate de idioma de producao depende dele")
	require.True(t, strings.HasPrefix(strings.ToLower(resp.Language), "pt"),
		"language retornado deve ser PT-BR, obtido=%q", resp.Language)
}
