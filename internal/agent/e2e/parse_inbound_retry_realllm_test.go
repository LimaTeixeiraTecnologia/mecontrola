//go:build integration

package e2e_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/providers/openrouter"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
)

const retryRealLLMAttempts = 10

type retryPhrase struct {
	text   string
	kind   intent.Kind
	accept func(intent.Intent) bool
	expect string
}

func realChainForModel(t *testing.T, slug valueobjects.ModelSlug) usecases.IntentInterpreter {
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
	return chain
}

func realParserWithRetry(t *testing.T, primary, retry valueobjects.ModelSlug) *usecases.ParseInbound {
	t.Helper()
	parser, err := usecases.NewParseInbound(realChainForModel(t, primary), realChainForModel(t, retry), 2000, noop.NewProvider())
	require.NoError(t, err)
	return parser
}

func realParserPrimaryOnly(t *testing.T, primary valueobjects.ModelSlug) *usecases.ParseInbound {
	t.Helper()
	parser, err := usecases.NewParseInbound(realChainForModel(t, primary), nil, 2000, noop.NewProvider())
	require.NoError(t, err)
	return parser
}

func retryBorderlinePhrases() []retryPhrase {
	pctEq := func(want int) func(intent.Intent) bool {
		return func(i intent.Intent) bool { return i.Percentage() == want && i.CategoryName() != "" }
	}
	dueDayEq := func(want int) func(intent.Intent) bool {
		return func(i intent.Intent) bool { p := i.DueDayPtr(); return p != nil && *p == want }
	}
	cardNamed := func(i intent.Intent) bool { return normalizeName(i.CardName()) != "" }

	return []retryPhrase{
		{"o vencimento do cartão itaú muda pro dia 10", intent.KindUpdateCard, dueDayEq(10), "update_card due_day=10"},
		{"troca o vencimento do cartão Itaú pro dia 10", intent.KindUpdateCard, dueDayEq(10), "update_card due_day=10"},
		{"apaga o cartão C6", intent.KindDeleteCard, cardNamed, "delete_card card=c6"},
		{"quero deixar lazer com 15%", intent.KindEditCategoryPercentage, pctEq(15), "edit_pct lazer=15"},
	}
}

func countRetryCorrect(parser *usecases.ParseInbound, p retryPhrase) (correct int, lastKind intent.Kind) {
	for attempt := 0; attempt < retryRealLLMAttempts; attempt++ {
		out, err := parser.Execute(context.Background(), usecases.ParseInboundInput{UserID: uuid.New(), Text: p.text})
		if err != nil {
			continue
		}
		lastKind = out.Intent.Kind()
		if out.Intent.Kind() == p.kind && p.accept(out.Intent) {
			correct++
		}
	}
	return correct, lastKind
}

func TestParseInbound_RealLLM_RetryOnUnknown_Borderline(t *testing.T) {
	if os.Getenv("RUN_REAL_LLM") == "" {
		t.Skip("set RUN_REAL_LLM=1 e exporte OPENROUTER_API_KEY para rodar a validacao real")
	}
	require.NotEmpty(t, os.Getenv("OPENROUTER_API_KEY"), "OPENROUTER_API_KEY ausente")

	primary := valueobjects.ModelSlugGeminiFlashLite()
	retry, err := valueobjects.NewModelSlug("mistralai/mistral-small-3.2-24b-instruct")
	require.NoError(t, err)

	phrases := retryBorderlinePhrases()
	primaryOnly := realParserPrimaryOnly(t, primary)
	withRetry := realParserWithRetry(t, primary, retry)

	type row struct {
		text      string
		expect    string
		solo      int
		withRetry int
	}
	rows := make([]row, 0, len(phrases))

	t.Log("==================== RETRY-ON-UNKNOWN: BORDERLINE ====================")
	t.Logf("primary=%s retry=%s attempts/frase=%d", primary.String(), retry.String(), retryRealLLMAttempts)

	totalSolo := 0
	totalRetry := 0
	for _, p := range phrases {
		solo, soloKind := countRetryCorrect(primaryOnly, p)
		comb, combKind := countRetryCorrect(withRetry, p)
		totalSolo += solo
		totalRetry += comb
		rows = append(rows, row{p.text, p.expect, solo, comb})
		t.Logf("phrase=%q expect=%s | gemini-solo=%d/%d (last=%s) | gemini+retry=%d/%d (last=%s)",
			p.text, p.expect, solo, retryRealLLMAttempts, soloKind.String(), comb, retryRealLLMAttempts, combKind.String())
	}
	t.Logf("TOTAL gemini-solo=%d/%d | gemini+retry=%d/%d",
		totalSolo, len(phrases)*retryRealLLMAttempts, totalRetry, len(phrases)*retryRealLLMAttempts)
	t.Log("=====================================================================")

	for _, r := range rows {
		require.GreaterOrEqualf(t, r.withRetry, r.solo,
			"retry regrediu em %q: gemini-solo=%d gemini+retry=%d (esperado=%s)", r.text, r.solo, r.withRetry, r.expect)
		require.GreaterOrEqualf(t, r.withRetry, retryRealLLMAttempts-1,
			"gemini+retry abaixo do limiar em %q: %d/%d (esperado=%s)", r.text, r.withRetry, retryRealLLMAttempts, r.expect)
	}

	require.GreaterOrEqualf(t, totalRetry, totalSolo,
		"retry nao pode regredir no agregado: gemini-solo=%d gemini+retry=%d", totalSolo, totalRetry)
}

func TestParseInbound_RealLLM_RetryRecoversGeminiUnknown(t *testing.T) {
	if os.Getenv("RUN_REAL_LLM") == "" {
		t.Skip("set RUN_REAL_LLM=1 e exporte OPENROUTER_API_KEY para rodar a validacao real")
	}
	require.NotEmpty(t, os.Getenv("OPENROUTER_API_KEY"), "OPENROUTER_API_KEY ausente")

	primary := valueobjects.ModelSlugGeminiFlashLite()
	retry, err := valueobjects.NewModelSlug("mistralai/mistral-small-3.2-24b-instruct")
	require.NoError(t, err)

	phrases := retryBorderlinePhrases()
	primaryOnly := realParserPrimaryOnly(t, primary)
	withRetry := realParserWithRetry(t, primary, retry)

	t.Log("================ RECOVERY: GEMINI-UNKNOWN -> RETRY ================")
	t.Logf("primary=%s retry=%s attempts/frase=%d", primary.String(), retry.String(), retryRealLLMAttempts)

	observedUnknown := 0
	recovered := 0
	for _, p := range phrases {
		phraseUnknown := 0
		phraseRecovered := 0
		for attempt := 0; attempt < retryRealLLMAttempts; attempt++ {
			soloOut, soloErr := primaryOnly.Execute(context.Background(), usecases.ParseInboundInput{UserID: uuid.New(), Text: p.text})
			if soloErr != nil {
				continue
			}
			if soloOut.Intent.Kind() != intent.KindUnknown {
				continue
			}
			phraseUnknown++
			combOut, combErr := withRetry.Execute(context.Background(), usecases.ParseInboundInput{UserID: uuid.New(), Text: p.text})
			if combErr != nil {
				continue
			}
			if combOut.Intent.Kind() == p.kind && p.accept(combOut.Intent) {
				phraseRecovered++
			}
		}
		observedUnknown += phraseUnknown
		recovered += phraseRecovered
		t.Logf("phrase=%q expect=%s | gemini-unknown observados=%d | recuperados-pelo-retry=%d",
			p.text, p.expect, phraseUnknown, phraseRecovered)
	}
	t.Logf("TOTAL gemini-unknown observados=%d | recuperados-pelo-retry=%d", observedUnknown, recovered)
	t.Log("=================================================================")

	if observedUnknown == 0 {
		t.Skip("nenhum gemini-unknown observado nesta execucao (modelo nao-deterministico); rode novamente para capturar a recuperacao")
	}
	require.Greaterf(t, recovered, 0,
		"retry nao recuperou nenhum dos %d gemini-unknown observados", observedUnknown)
	require.GreaterOrEqualf(t, recovered*2, observedUnknown,
		"retry recuperou menos da metade dos gemini-unknown observados: recuperados=%d observados=%d", recovered, observedUnknown)
}
