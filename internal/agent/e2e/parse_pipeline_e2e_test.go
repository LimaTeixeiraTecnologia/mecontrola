//go:build e2e

package e2e_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
	agentbinding "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/binding"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/providers/openrouter"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions"
)

func newMockOpenRouterChain(t *testing.T, content string, status int) *appservices.FallbackChain {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if status != http.StatusOK {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":{"message":"boom"}}`))
			return
		}
		body := map[string]any{
			"choices": []map[string]any{{
				"message":       map[string]any{"role": "assistant", "content": content},
				"finish_reason": "stop",
			}},
			"usage": map[string]any{"prompt_tokens": 120, "completion_tokens": 18},
		}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(body))
	}))
	t.Cleanup(server.Close)

	client, err := httpclient.NewClient(noop.NewProvider(),
		httpclient.WithBaseURL(server.URL),
		httpclient.WithTarget("openrouter_parse_mock"),
		httpclient.WithTimeout(3*time.Second),
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

	breaker := appservices.NewCircuitBreaker(appservices.CircuitBreakerConfig{MaxFailures: 5, FailureWindow: 30 * time.Second, OpenDuration: 60 * time.Second})
	chain, err := appservices.NewFallbackChain([]appinterfaces.LLMProvider{provider}, breaker, noop.NewProvider())
	require.NoError(t, err)
	return chain
}

func TestParsePipeline_RealParser_PersistsExpense_E2E(t *testing.T) {
	db, _ := postgres.NewTestDatabase(t)
	o11y := noop.NewProvider()
	ctx := context.Background()

	cfg, err := configs.LoadConfig("../../..")
	require.NoError(t, err)
	cfg.TransactionsConfig.Enabled = true

	authMW := func(h http.Handler) http.Handler { return h }
	catModule := categories.NewCategoriesModule(db, o11y, authMW)
	cardModule, err := card.NewCardModule(ctx, cfg, o11y, db, authMW, nil, nil)
	require.NoError(t, err)
	txModule, err := transactions.NewTransactionsModule(cfg, o11y, db, cardModule, catModule, authMW)
	require.NoError(t, err)

	logTx := usecases.NewRecordTransactionFromAgent(
		catModule.SearchDictionaryUC,
		agentbinding.NewTransactionCreatorAdapter(txModule.CreateTransactionUC),
		o11y,
	)
	logCardPurchase := usecases.NewRecordCardPurchaseFromAgent(
		catModule.SearchDictionaryUC,
		agentbinding.NewCardPurchaseCreatorAdapter(cardModule.ListCardsUC, txModule.CreateCardPurchaseUC),
		o11y,
	)
	expLogger := agentbinding.NewTransactionLoggerAdapter(logTx)

	const canonicalExpenseJSON = `{"kind":"record_expense","amount_cents":5800,"merchant":"ifood","category_hint":"delivery"}`
	chain := newMockOpenRouterChain(t, canonicalExpenseJSON, http.StatusOK)
	parser, err := usecases.NewParseInbound(chain, nil, 2000, o11y)
	require.NoError(t, err)

	transactionLister := agentbinding.NewTransactionListerAdapter(txModule.ListTransactionsUC)
	transactionSearcher := agentbinding.NewTransactionSearcherAdapter(txModule.SearchTransactionsUC)
	lastEditor := agentbinding.NewLastTransactionEditorAdapter(txModule.GetTransactionUC, txModule.UpdateTransactionUC)
	lastDeleter := agentbinding.NewLastTransactionDeleterAdapter(txModule.DeleteTransactionUC)
	cardDeleter := agentbinding.NewCardDeleterAdapter(cardModule.ListCardsUC, cardModule.SoftDeleteCardUC)
	kernelDeps, _, err := buildConfirmKernelDeps(
		o11y,
		db,
		cfg,
		transactionLister,
		transactionSearcher,
		lastEditor,
		lastDeleter,
		cardModule.ListCardsUC,
		cardDeleter,
		agentbinding.NewKernelCategoryResolver(catModule.SearchDictionaryUC),
		agentbinding.NewKernelPersistFunc(expLogger, agentbinding.NewCardPurchaseLoggerAdapter(logCardPurchase)),
	)
	require.NoError(t, err)

	gateway := &CapturingGateway{}
	router, err := appservices.NewIntentRouter(o11y, appservices.IntentRouterDeps{
		Parser:          &parserAdapter{uc: parser},
		ExpenseRecorder: expLogger,
		Fallback:        &StubFallback{},
		WhatsAppGateway: gateway,
		Location:        time.UTC,
		Kernel:          kernelDeps,
	})
	require.NoError(t, err)

	waNumber := "+5511977776666"
	userID := SeedActiveUserWA(t, db, waNumber)
	principal := appservices.Principal{UserID: userID}

	result := router.RouteWhatsApp(ctx, principal, appservices.InboundMessage{
		Text:       "gastei 58 no ifood",
		WhatsAppTo: waNumber,
		MessageID:  "wamid.parse.e2e." + uuid.New().String(),
	})
	require.Equal(t, tools.OutcomeRouted, result.Outcome, "gasto deve ser roteado pelo parser real")

	var (
		total       int
		amountCents int64
	)
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	require.NoError(t, db.QueryRowContext(queryCtx,
		`SELECT count(*), COALESCE(max(amount_cents), 0)
		   FROM mecontrola.transactions
		  WHERE user_id = $1 AND deleted_at IS NULL`,
		userID,
	).Scan(&total, &amountCents))
	require.Equal(t, 1, total, "parser real deve ter persistido exatamente 1 transacao")
	require.Equal(t, int64(5800), amountCents, "valor deve ter sido extraido do JSON do mock pelo parser real")

	var outboxTotal int
	require.NoError(t, db.QueryRowContext(queryCtx,
		`SELECT count(*) FROM mecontrola.outbox_events
		  WHERE event_type = $1 AND aggregate_user_id = $2`,
		transactionCreatedType, userID,
	).Scan(&outboxTotal))
	require.GreaterOrEqual(t, outboxTotal, 1, "evento transactions.transaction.created.v1 deve estar no outbox")
}

func TestParsePipeline_RealParser_ProviderDownFallsBack_E2E(t *testing.T) {
	db, _ := postgres.NewTestDatabase(t)
	o11y := noop.NewProvider()
	ctx := context.Background()

	cfg, err := configs.LoadConfig("../../..")
	require.NoError(t, err)
	cfg.TransactionsConfig.Enabled = true

	authMW := func(h http.Handler) http.Handler { return h }
	catModule := categories.NewCategoriesModule(db, o11y, authMW)
	cardModule, err := card.NewCardModule(ctx, cfg, o11y, db, authMW, nil, nil)
	require.NoError(t, err)
	txModule, err := transactions.NewTransactionsModule(cfg, o11y, db, cardModule, catModule, authMW)
	require.NoError(t, err)

	logTx := usecases.NewRecordTransactionFromAgent(
		catModule.SearchDictionaryUC,
		agentbinding.NewTransactionCreatorAdapter(txModule.CreateTransactionUC),
		o11y,
	)

	chain := newMockOpenRouterChain(t, "", http.StatusInternalServerError)
	parser, err := usecases.NewParseInbound(chain, nil, 2000, o11y)
	require.NoError(t, err)

	gateway := &CapturingGateway{}
	router, err := appservices.NewIntentRouter(o11y, appservices.IntentRouterDeps{
		Parser:          &parserAdapter{uc: parser},
		ExpenseRecorder: agentbinding.NewTransactionLoggerAdapter(logTx),
		Fallback:        &StubFallback{},
		WhatsAppGateway: gateway,
		Location:        time.UTC,
	})
	require.NoError(t, err)

	waNumber := "+5511966665555"
	userID := SeedActiveUserWA(t, db, waNumber)
	principal := appservices.Principal{UserID: userID}

	result := router.RouteWhatsApp(ctx, principal, appservices.InboundMessage{
		Text:       "gastei 58 no ifood",
		WhatsAppTo: waNumber,
		MessageID:  "wamid.parse.fb." + uuid.New().String(),
	})
	require.Equal(t, tools.OutcomeFallback, result.Outcome, "provider em erro deve cair em fallback")

	var total int
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	require.NoError(t, db.QueryRowContext(queryCtx,
		`SELECT count(*) FROM mecontrola.transactions WHERE user_id = $1 AND deleted_at IS NULL`,
		userID,
	).Scan(&total))
	require.Equal(t, 0, total, "fallback nao deve persistir transacao")
}
