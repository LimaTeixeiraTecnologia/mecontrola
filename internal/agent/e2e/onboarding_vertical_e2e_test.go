//go:build e2e

package e2e_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	appusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
	agentbinding "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/binding"
	agentevents "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/events"
	agentonboarding "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/onboarding"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/providers/openrouter"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories"
	identityauth "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	identityuc "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
	identityrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories"
	onbusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	onbentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	onbvalueobjects "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	onbrepositories "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	deduppostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/dedup/postgres"
	wadispatcher "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/dispatcher"
	wahandlers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/handlers"
	wapayload "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/payload"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/ratelimit"
	wasignature "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/signature"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions"
)

type scriptedToolCall struct {
	name string
	args string
}

type mockOpenRouterScript struct {
	mu     sync.Mutex
	script map[string]scriptedToolCall
}

func (m *mockOpenRouterScript) handle(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	raw, _ := io.ReadAll(r.Body)
	var decoded struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	_ = json.Unmarshal(raw, &decoded)

	lastUser := ""
	for _, msg := range decoded.Messages {
		if msg.Role == "user" {
			lastUser = msg.Content
		}
	}
	lower := strings.ToLower(lastUser)

	w.Header().Set("Content-Type", "application/json")

	for substr, call := range m.script {
		if strings.Contains(lower, substr) {
			body := map[string]any{
				"choices": []map[string]any{{
					"message": map[string]any{
						"role": "assistant",
						"tool_calls": []map[string]any{{
							"id":   "c1",
							"type": "function",
							"function": map[string]any{
								"name":      call.name,
								"arguments": call.args,
							},
						}},
					},
					"finish_reason": "tool_calls",
				}},
				"usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 5},
			}
			_ = json.NewEncoder(w).Encode(body)
			return
		}
	}

	body := map[string]any{
		"choices": []map[string]any{{
			"message":       map[string]any{"role": "assistant", "content": "Vamos continuar? 🙂"},
			"finish_reason": "stop",
		}},
		"usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 5},
	}
	_ = json.NewEncoder(w).Encode(body)
}

func newScriptedOpenRouterChain(t *testing.T) *appservices.FallbackChain {
	t.Helper()
	script := &mockOpenRouterScript{
		script: map[string]scriptedToolCall{
			"viagem":   {name: "save_onboarding_objective", args: `{"objective":"fazer uma viagem"}`},
			"5000":     {name: "save_onboarding_income", args: `{"income_cents":500000}`},
			"nubank":   {name: "save_onboarding_card", args: `{"nickname":"nubank","due_day":17}`},
			"distribu": {name: "save_onboarding_budget_splits", args: `{"allocations":[{"root_slug":"expense.custo_fixo","amount_cents":200000},{"root_slug":"expense.conhecimento","amount_cents":50000},{"root_slug":"expense.prazeres","amount_cents":75000},{"root_slug":"expense.metas","amount_cents":100000},{"root_slug":"expense.liberdade_financeira","amount_cents":75000}]}`},
			"mercado":  {name: "record_transaction", args: `{"direction":"outcome","amount_cents":3500,"merchant":"mercado","category_hint":"mercado"}`},
		},
	}
	server := httptest.NewServer(http.HandlerFunc(script.handle))
	t.Cleanup(server.Close)

	client, err := httpclient.NewClient(noop.NewProvider(),
		httpclient.WithBaseURL(server.URL),
		httpclient.WithTarget("openrouter_onboarding_mock"),
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

type fakeOutboxPublisher struct{}

func (fakeOutboxPublisher) Publish(_ context.Context, _ outbox.Event) error { return nil }

func TestOnboardingVertical_E2E(t *testing.T) {
	db, _ := postgres.NewTestDatabase(t)
	o11y := noop.NewProvider()
	ctx := context.Background()

	cfg, err := configs.LoadConfig("../../..")
	require.NoError(t, err)
	cfg.TransactionsConfig.Enabled = true

	secret := "test-secret-onboarding-vertical"
	waNumber := "+5511955554444"
	waFrom := "5511955554444"

	limiter := ratelimit.New(o11y)
	require.NoError(t, limiter.Start(ctx))
	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), ratelimit.DefaultShutdownTimeout)
		defer cancel()
		_ = limiter.Shutdown(shutdownCtx)
	})

	authMW := func(h http.Handler) http.Handler { return h }
	catModule := categories.NewCategoriesModule(db, o11y, authMW)
	cardModule, err := card.NewCardModule(ctx, cfg, o11y, db, authMW, nil, nil)
	require.NoError(t, err)
	txModule, err := transactions.NewTransactionsModule(cfg, o11y, db, cardModule, catModule, authMW)
	require.NoError(t, err)

	logTx := appusecases.NewRecordTransactionFromAgent(
		catModule.SearchDictionaryUC,
		agentbinding.NewTransactionCreatorAdapter(txModule.CreateTransactionUC),
		o11y,
	)
	expLogger := agentbinding.NewTransactionLoggerAdapter(logTx)

	onbfactory := onbrepositories.NewRepositoryFactory(o11y)
	sessionRepo := onbfactory.OnboardingSessionRepository(db)
	idGen := id.NewUUIDGenerator()
	var fakePublisher outbox.Publisher = fakeOutboxPublisher{}

	getContext := onbusecases.NewGetOnboardingContext(sessionRepo, o11y)
	saveObjective := onbusecases.NewSaveOnboardingObjective(uow.NewUnitOfWork(db), onbfactory, o11y)
	saveIncome := onbusecases.NewSaveOnboardingIncome(uow.NewUnitOfWork(db), onbfactory, fakePublisher, idGen, o11y)
	saveCard := onbusecases.NewSaveOnboardingCard(uow.NewUnitOfWork(db), onbfactory, fakePublisher, idGen, o11y, nil)
	saveSplits := onbusecases.NewSaveOnboardingBudgetSplits(uow.NewUnitOfWork(db), onbfactory, fakePublisher, idGen, o11y)
	markFirstTx := onbusecases.NewMarkFirstTransactionRecorded(uow.NewUnitOfWork(db), onbfactory, o11y)
	complete := onbusecases.NewCompleteOnboardingSession(uow.NewUnitOfWork(db), onbfactory, fakePublisher, idGen, o11y)
	setPhase := onbusecases.NewSetOnboardingPhase(uow.NewUnitOfWork(db), onbfactory, o11y)

	reader := agentonboarding.NewOnboardingStateReader(getContext)
	phaseSetter := agentonboarding.NewOnboardingPhaseSetter(setPhase)
	require.NotNil(t, phaseSetter)
	dispatcher := agentonboarding.NewOnboardingToolDispatcher(saveObjective, saveIncome, saveCard, saveSplits, markFirstTx, complete, getContext, nil, expLogger)
	chain := newScriptedOpenRouterChain(t)
	runTurn, err := appusecases.NewRunOnboardingTurn(chain, reader, dispatcher, phaseSetter, 512, o11y, nil)
	require.NoError(t, err)
	runner := agentonboarding.NewOnboardingTurnRunnerAdapter(runTurn)

	gateway := &CapturingGateway{}

	publisher := outbox.NewPostgresPublisher(
		outbox.NewPostgresStorage(db),
		configs.OutboxConfig{RetryMaxAttempts: 3},
	)

	intentRouter, err := appservices.NewIntentRouter(o11y, appservices.IntentRouterDeps{
		Parser:           NewStubParser(map[string]intent.Intent{}, nil),
		Fallback:         &StubFallback{},
		WhatsAppGateway:  gateway,
		ExpenseRecorder:  expLogger,
		OnboardingRunner: runner,
		EventPublisher:   agentevents.NewIntentEventPublisher(publisher, o11y),
		Location:         time.UTC,
	})
	require.NoError(t, err)

	agentRoute := func(c context.Context, msg wapayload.Message) wadispatcher.RouteOutcome {
		principal, ok := identityauth.FromContext(c)
		if !ok {
			return wadispatcher.OutcomeAgent
		}
		_ = intentRouter.RouteWhatsApp(c, appservices.Principal{UserID: principal.UserID}, appservices.InboundMessage{
			Text:       msg.Text,
			WhatsAppTo: msg.From,
			MessageID:  msg.WAMID,
		})
		return wadispatcher.OutcomeAgent
	}
	onboardingRoute := func(_ context.Context, _ wapayload.Message) wadispatcher.RouteOutcome {
		return wadispatcher.OutcomeOnboarding
	}

	factory := identityrepos.NewRepositoryFactory(o11y)
	establishUC := identityuc.NewEstablishPrincipal(uow.NewUnitOfWork(db), factory, publisher, o11y)
	dedupRepo := deduppostgres.NewMessageRepository(o11y, db)

	disp := wadispatcher.New(dedupRepo, establishUC, limiter, publisher, onboardingRoute, agentRoute, o11y)
	inboundHandler := wahandlers.NewInboundHandler(disp, o11y)

	r := chi.NewRouter()
	r.Route("/api/v1/whatsapp", func(sub chi.Router) {
		sub.With(wasignature.Compose(secret, "", nil)).Post("/inbound", inboundHandler.Handle)
	})
	server := httptest.NewServer(r)
	t.Cleanup(server.Close)

	userID := SeedActiveUserWA(t, db, waNumber)
	session, sErr := onbentities.NewOnboardingSession(userID, onbentities.OnboardingChannelWhatsApp, onbvalueobjects.OnboardingStateAwaitingIncome, time.Now().UTC())
	require.NoError(t, sErr)
	require.NoError(t, sessionRepo.Upsert(ctx, session))

	e := newAgentE2ECtx(t, server, db, gateway, nil, nil, nil, nil, secret, waNumber, waFrom, userID)

	beforeTx, err := e.countTransactions(userID)
	require.NoError(t, err)

	postOnboarding(t, e, "oi")
	requireLastReplyContains(t, gateway, "Eu sou o *MeControla*")
	requireOnboardingPhase(t, db, userID, "welcome")

	progression := []struct {
		phase   string
		snippet string
	}{
		{"methodology_1", "Custo Fixo"},
		{"methodology_2", "Conhecimento"},
		{"methodology_3", "Prazeres"},
		{"methodology_4", "Metas"},
		{"methodology_5", "Liberdade Financeira"},
	}
	for _, step := range progression {
		postOnboarding(t, e, "sim")
		requireLastReplyContains(t, gateway, step.snippet)
		requireOnboardingPhase(t, db, userID, step.phase)
	}

	postOnboarding(t, e, "sim")
	requireLastReplyContains(t, gateway, "objetivo principal")
	requireOnboardingPhase(t, db, userID, "objective")

	postOnboarding(t, e, "quero fazer uma viagem")
	require.Equal(t, "fazer uma viagem", onbPayloadString(t, db, userID, "objective"))
	requireLastReplyContains(t, gateway, "🎯 Anotado: seu foco é *fazer uma viagem*.")
	requireLastReplyContains(t, gateway, "orçamento mensal")
	requireOnboardingPhase(t, db, userID, "income")

	postOnboarding(t, e, "ganho 5000 por mes")
	require.Equal(t, int64(500000), onbPayloadInt(t, db, userID, "income_cents"))
	requireLastReplyContains(t, gateway, "✅ Orçamento de *R$ 5.000,00* registrado!")
	requireLastReplyContains(t, gateway, "cartão de crédito")
	requireOnboardingPhase(t, db, userID, "cards")

	postOnboarding(t, e, "uso o nubank, vence dia 17")
	require.Equal(t, 1, onbCardsLen(t, db, userID))
	requireLastReplyContains(t, gateway, "💳 Cartão *nubank* salvo (vence dia 17")
	requireOnboardingPhase(t, db, userID, "cards")

	postOnboarding(t, e, "não, só esse")
	requireLastReplyContains(t, gateway, "distribuir seu orçamento")
	requireOnboardingPhase(t, db, userID, "splits")

	postOnboarding(t, e, "distribui assim")
	require.Equal(t, 5, onbSplitsLen(t, db, userID))
	requireLastReplyContains(t, gateway, "✅ *Distribuição salva!*")
	requireLastReplyContains(t, gateway, "Seu plano:")
	requireOnboardingPhase(t, db, userID, "summary")

	postOnboarding(t, e, "tá perfeito")
	requireLastReplyContains(t, gateway, "primeiro lançamento")
	requireOnboardingPhase(t, db, userID, "first_tx")

	postOnboarding(t, e, "gastei 35 no mercado")
	afterTx, err := e.countTransactions(userID)
	require.NoError(t, err)
	require.Equal(t, beforeTx+1, afterTx, "primeira transacao real deve ter sido persistida")
	require.Equal(t, "active", onbState(t, db, userID))
	last, ok := gateway.LastReply()
	require.True(t, ok)
	require.Contains(t, last.Text, "🏆 Boa! Registrei")
	require.Contains(t, last.Text, "🎉 *Onboarding concluído!*")
}

func postOnboarding(t *testing.T, e *agentE2ECtx, text string) {
	t.Helper()
	require.NoError(t, e.postWebhook(text, "wamid.onb."+uuid.New().String()))
}

func requireLastReplyContains(t *testing.T, gateway *CapturingGateway, substr string) {
	t.Helper()
	last, ok := gateway.LastReply()
	require.True(t, ok, "esperava resposta no gateway")
	require.Contains(t, last.Text, substr)
}

func onbState(t *testing.T, db *sqlx.DB, userID uuid.UUID) string {
	t.Helper()
	queryCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var state string
	require.NoError(t, db.QueryRowContext(queryCtx,
		`SELECT state FROM mecontrola.onboarding_sessions WHERE user_id = $1`, userID).Scan(&state))
	return state
}

func onbPayloadString(t *testing.T, db *sqlx.DB, userID uuid.UUID, key string) string {
	t.Helper()
	queryCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var value string
	require.NoError(t, db.QueryRowContext(queryCtx,
		`SELECT payload->>$2 FROM mecontrola.onboarding_sessions WHERE user_id = $1`, userID, key).Scan(&value))
	return value
}

func onbPayloadInt(t *testing.T, db *sqlx.DB, userID uuid.UUID, key string) int64 {
	t.Helper()
	queryCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var value int64
	require.NoError(t, db.QueryRowContext(queryCtx,
		`SELECT (payload->>$2)::bigint FROM mecontrola.onboarding_sessions WHERE user_id = $1`, userID, key).Scan(&value))
	return value
}

func onbCardsLen(t *testing.T, db *sqlx.DB, userID uuid.UUID) int {
	t.Helper()
	queryCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var n int
	require.NoError(t, db.QueryRowContext(queryCtx,
		`SELECT jsonb_array_length(payload->'cards') FROM mecontrola.onboarding_sessions WHERE user_id = $1`, userID).Scan(&n))
	return n
}

func onbSplitsLen(t *testing.T, db *sqlx.DB, userID uuid.UUID) int {
	t.Helper()
	queryCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var n int
	require.NoError(t, db.QueryRowContext(queryCtx,
		`SELECT jsonb_array_length(payload->'custom_split') FROM mecontrola.onboarding_sessions WHERE user_id = $1`, userID).Scan(&n))
	return n
}
