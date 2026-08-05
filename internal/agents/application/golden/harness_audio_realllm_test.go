//go:build integration

package golden

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	agentsapp "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/agents"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
)

type audioGroupResult struct {
	group AudioGroup
	hits  int
	total int
	fails []string
}

func (r audioGroupResult) ratio() float64 {
	if r.total == 0 {
		return 1
	}
	return float64(r.hits) / float64(r.total)
}

type GoldenAudioRealLLMSuite struct {
	suite.Suite
	provider llm.Provider
}

func TestGoldenAudioRealLLMSuite(t *testing.T) {
	suite.Run(t, new(GoldenAudioRealLLMSuite))
}

func (s *GoldenAudioRealLLMSuite) SetupSuite() {
	s.provider = buildGoldenHarnessProvider(s.T())
}

func (s *GoldenAudioRealLLMSuite) executorFor(tools []tool.ToolHandle) AgentExecutor {
	provider := s.provider
	return func(ctx context.Context, messages []llm.Message) (agent.Result, error) {
		obs := fake.NewProvider()
		a := agentsapp.BuildMeControlaAgent(provider, tools, nil, obs, 0)
		if len(messages) > 0 && messages[0].Role == "system" {
			messages[0].Content = a.Instructions() + "\n\n" + messages[0].Content
		}
		ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
		defer cancel()
		return a.Execute(ctx, agent.Request{
			AgentID:   agentsapp.MecontrolaAgentID,
			Messages:  messages,
			MaxTokens: 1024,
		})
	}
}

func (s *GoldenAudioRealLLMSuite) TestGoldenAudioPairedGate() {
	t := s.T()

	results := map[AudioGroup]*audioGroupResult{}
	for _, group := range AllAudioGroups() {
		results[group] = &audioGroupResult{group: group}
	}

	for _, ac := range AudioPairedCases() {
		confidence := 0.95
		decision := usecases.DecideAudioTranscription(input.AudioDecisionInput{
			Model:         "openai/whisper-large-v3",
			Text:          ac.SpokenText,
			Language:      "pt",
			Confidence:    &confidence,
			MinConfidence: 0.80,
		})
		require.Equalf(t, usecases.AudioOutcomeApproved, decision.Outcome,
			"caso de audio %q deveria decidir approved a partir do texto simulado antes do gate real de LLM", ac.Name)
		require.True(t, decision.Outcome.AllowsHandleInbound())

		textCase, ok := ac.ResolveTextCase()
		require.Truef(t, ok,
			"RF-30: caso de audio %q referencia caso textual inexistente %q", ac.Name, ac.TextCaseName)
		textCase.Input = decision.CanonicalText

		for attempt := 1; attempt <= goldenRepeatsPerCase; attempt++ {
			s.Run(fmt.Sprintf("%s/%d", ac.Name, attempt), func() {
				var captured []CapturedToolCall
				sink := func(name string, args map[string]any) {
					captured = append(captured, CapturedToolCall{Tool: name, Args: args})
				}
				executor := s.executorFor(goldenToolsFor(textCase, sink))
				outcome := EvaluateCaseWithCapture(context.Background(), executor, textCase, func() []CapturedToolCall { return captured })
				result := results[ac.Group]
				result.total++
				if outcome.Passed {
					result.hits++
				} else {
					result.fails = append(result.fails, ac.Name+": "+outcome.Detail)
					t.Logf("[audio/%s/%s attempt=%d] FALHOU: %s", ac.Group, ac.Name, attempt, outcome.Detail)
				}
			})
		}
	}

	failed := false
	for _, group := range AllAudioGroups() {
		r := results[group]
		t.Logf("audio_group=%s hits=%d total=%d ratio=%.4f", group, r.hits, r.total, r.ratio())
		if r.ratio() < goldenGateThreshold {
			failed = true
			t.Logf("audio_group=%s ABAIXO do gate %.2f; falhas=%v", group, goldenGateThreshold, r.fails)
		}
	}
	require.False(t, failed, "RF-30/RF-32/RF-34: um ou mais grupos do golden audio pareado ficaram abaixo do gate %.2f", goldenGateThreshold)
}

func realGoldenAudioFormat(t *testing.T, path string) llm.AudioFormat {
	t.Helper()
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ogg", ".opus":
		return llm.AudioFormatOGG
	case ".m4a", ".aac", ".mp4":
		return llm.AudioFormatM4A
	default:
		t.Fatalf("STT_REAL_AUDIO_FIXTURE com extensao nao suportada: %s", path)
		return ""
	}
}

func TestGoldenAudioRealSTT(t *testing.T) {
	if os.Getenv("RUN_REAL_STT") != "1" {
		t.Skip("RUN_REAL_STT=1 obrigatório para a suite real de STT do golden set")
	}

	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		t.Fatal("RUN_REAL_STT=1 exige OPENROUTER_API_KEY; suite real de STT não pode ser marcada como atendida sem credencial")
	}

	audioPath := os.Getenv("STT_REAL_AUDIO_FIXTURE")
	if audioPath == "" {
		t.Fatal("RUN_REAL_STT=1 exige STT_REAL_AUDIO_FIXTURE apontando para áudio PT-BR real; " +
			"o gate real do golden não pode ser considerado atendido sem fixture executada")
	}
	audio, err := os.ReadFile(audioPath)
	require.NoError(t, err)

	baseURL := os.Getenv("OPENROUTER_BASE_URL")
	if baseURL == "" {
		baseURL = "https://openrouter.ai"
	}
	client, err := httpclient.NewClient(fake.NewProvider(),
		httpclient.WithBaseURL(baseURL),
		httpclient.WithTarget("openrouter_stt_golden"),
		httpclient.WithTimeout(30*time.Second),
	)
	require.NoError(t, err)

	transcriber := llm.NewOpenRouterTranscriber(client, llm.Config{
		BaseURL:     baseURL,
		APIKey:      apiKey,
		HTTPReferer: "https://github.com/LimaTeixeiraTecnologia/mecontrola",
		XTitle:      "mecontrola-golden-audio-realstt",
		STTTimeout:  20 * time.Second,
	}, fake.NewProvider())

	resp, err := transcriber.Transcribe(context.Background(), llm.TranscriptionRequest{
		Model:           "openai/whisper-large-v3",
		Audio:           audio,
		Format:          realGoldenAudioFormat(t, audioPath),
		Language:        "pt",
		Temperature:     0,
		DurationMs:      1000,
		MaxCostMicrousd: 2000,
	})
	require.NoError(t, err)
	require.Equal(t, "openrouter", resp.Provider)

	decision := usecases.DecideAudioTranscription(input.AudioDecisionInput{
		Model:         "openai/whisper-large-v3",
		Text:          resp.Text,
		Language:      resp.Language,
		Truncated:     resp.Truncated,
		Confidence:    resp.Confidence,
		MinConfidence: 0.80,
	})
	require.Equal(t, usecases.AudioOutcomeApproved, decision.Outcome,
		"RF-30/RF-36: transcricao real de fixture PT-BR valida deve ser aprovada pelo decisor")
}
