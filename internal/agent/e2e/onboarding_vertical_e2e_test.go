//go:build e2e

package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
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
	agentrepo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories"
	identityauth "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	identityuc "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
	identityrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories"
	onbusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	onbentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
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

type scriptedResponse struct {
	toolName string
	toolArgs string
	content  string
}

type mockOpenRouterScript struct {
	mu     sync.Mutex
	script map[string]scriptedResponse
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
	fmt.Printf("[MOCK] lastUser=%q lower=%q\n", lastUser, lower)

	w.Header().Set("Content-Type", "application/json")

	for substr, resp := range m.script {
		if strings.Contains(lower, substr) {
			message := map[string]any{"role": "assistant"}
			finishReason := "stop"
			if resp.toolName != "" {
				finishReason = "tool_calls"
				message["tool_calls"] = []map[string]any{{
					"id":   "c1",
					"type": "function",
					"function": map[string]any{
						"name":      resp.toolName,
						"arguments": resp.toolArgs,
					},
				}}
			}
			if resp.content != "" {
				message["content"] = resp.content
			}
			body := map[string]any{
				"choices": []map[string]any{{
					"message":       message,
					"finish_reason": finishReason,
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
		script: map[string]scriptedResponse{
			"inicie o onboarding": {content: "👋 Oi! Eu sou o *MeControla*, seu parceiro financeiro."},
			"viagem":              {toolName: "save_onboarding_objective", toolArgs: `{"objective":"fazer uma viagem"}`},
			"5000":                {toolName: "save_onboarding_income", toolArgs: `{"income_cents":500000}`},
			"mercado":             {toolName: "record_transaction", toolArgs: `{"direction":"outcome","amount_cents":3500,"merchant":"mercado","category_hint":"mercado"}`},
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
	dispatcher := agentonboarding.NewOnboardingToolDispatcher(saveObjective, saveIncome, saveCard, saveSplits, markFirstTx, complete, expLogger)
	chain := newScriptedOpenRouterChain(t)
	agentSessionRepo := agentrepo.NewRepositoryFactory(o11y).AgentSessionRepository(db)
	v2session := agentbinding.NewOnboardingSessionGateway(agentSessionRepo)
	runTurn, err := appusecases.NewRunOnboardingTurn(chain, reader, dispatcher, phaseSetter, 512, o11y, nil, fakeSplitSuggester{}, v2session)
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
	session, sErr := onbentities.NewOnboardingSession(userID, onbentities.OnboardingChannelWhatsApp, time.Now().UTC())
	require.NoError(t, sErr)
	require.NoError(t, sessionRepo.Upsert(ctx, session))

	e := newAgentE2ECtx(t, server, db, gateway, nil, nil, nil, nil, secret, waNumber, waFrom, userID)

	beforeTx, err := e.countTransactions(userID)
	require.NoError(t, err)

	postOnboarding(t, e, "oi")
	requireLastReplyContains(t, gateway, "Eu sou o *MeControla*")
	requireOnboardingPhase(t, db, userID, "objective")

	postOnboarding(t, e, "quero fazer uma viagem")
	require.Equal(t, "fazer uma viagem", onbPayloadString(t, db, userID, "objective"))
	requireOnboardingPhase(t, db, userID, "budget")

	postOnboarding(t, e, "ganho 5000 por mes")
	require.Equal(t, int64(500000), onbPayloadInt(t, db, userID, "income_cents"))
	requireOnboardingPhase(t, db, userID, "cards")

	postOnboarding(t, e, "não uso cartão")
	requireOnboardingPhase(t, db, userID, "financial_plan")

	postOnboarding(t, e, "sim")
	require.Equal(t, 5, onbSplitsLen(t, db, userID))
	requireOnboardingPhase(t, db, userID, "first_tx")

	postOnboarding(t, e, "gastei 35 no mercado")
	afterTx, err := e.countTransactions(userID)
	require.NoError(t, err)
	require.Equal(t, beforeTx+1, afterTx, "primeira transacao real deve ter sido persistida")
	require.Equal(t, "active", onbState(t, db, userID))
	last, ok := gateway.LastReply()
	require.True(t, ok)
	require.Contains(t, last.Text, "Onboarding concluído")
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
