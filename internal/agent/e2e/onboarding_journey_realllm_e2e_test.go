//go:build e2e && integration

package e2e_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	agentwf "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
	agentonboarding "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/onboarding"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets"
	cardusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/usecases"
	cardconsumers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/infrastructure/messaging/database/consumers"
	cardrepo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding"
	onbusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	onbentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
	platformevents "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/idempotency"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
	wfpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow/infrastructure/postgres"
)

const onboardingE2ECardOffsetDays = 10

func TestOnboardingJourney_RealLLM_FullEightPhases_E2E(t *testing.T) {
	if os.Getenv("RUN_REAL_LLM") == "" {
		t.Skip("set RUN_REAL_LLM=1 e exporte OPENROUTER_API_KEY para a jornada real")
	}
	require.NotEmpty(t, os.Getenv("OPENROUTER_API_KEY"), "OPENROUTER_API_KEY ausente")

	db, _ := postgres.NewTestDatabase(t)
	o11y := noop.NewProvider()
	ctx := context.Background()
	authMW := func(h http.Handler) http.Handler { return h }

	cfg := &configs.Config{}
	identityModule, err := identity.NewIdentityModule(cfg, o11y, db)
	require.NoError(t, err)
	catModule := categories.NewCategoriesModule(db, o11y, authMW)
	budgetsModule, err := budgets.NewBudgetsModule(cfg, o11y, db, catModule, authMW, nil, nil)
	require.NoError(t, err)

	cardFactory := cardrepo.NewRepositoryFactory(o11y)
	cardIdem := idempotency.NewPostgresStorage(db)
	createCard := cardusecases.NewCreateCard(uow.NewUnitOfWork(db), cardFactory, cardIdem, o11y)
	cardConsumer := cardconsumers.NewOnboardingCardConsumer(createCard, cardIdem, o11y)

	onbCfg := configs.OnboardingConfig{
		TokenEncryptionKey:     "0123456789abcdef0123456789abcdef",
		CardClosingOffsetDays:  onboardingE2ECardOffsetDays,
		AbandonmentTTLHours:    48,
		AbandonmentJobSchedule: "@hourly",
		AbandonmentBatchSize:   100,
	}
	mod, err := onboarding.NewOnboardingModule(db, onbCfg,
		configs.WhatsAppConfig{PhoneNumberID: "123456789", AccessToken: "fake-token"},
		configs.OutboxConfig{RetryMaxAttempts: 3},
		configs.EmailConfig{Provider: "smtp", SMTPHost: "smtp.example.com", SMTPPort: 587, FromAddress: "t@t.test", FromName: "T"},
		identityModule, o11y,
	)
	require.NoError(t, err)

	provider := realProviderForSlug(t, valueobjects.ModelSlugGeminiFlashLite())
	breaker := appservices.NewCircuitBreaker(appservices.CircuitBreakerConfig{MaxFailures: 5, FailureWindow: 30 * time.Second, OpenDuration: 60 * time.Second})
	chain, err := appservices.NewFallbackChain([]appinterfaces.LLMProvider{provider}, breaker, o11y)
	require.NoError(t, err)
	interpreter := agentonboarding.NewOnboardingInterpreter(chain, 512)

	deps := agentwf.OnboardingDeps{
		Interpreter:      interpreter,
		WelcomeMarker:    agentonboarding.NewWelcomeMarkerBinding(mod.MarkWelcomeSent),
		ObjectiveSaver:   agentonboarding.NewObjectiveSaverBinding(mod.SaveOnboardingObjective),
		IncomeSaver:      agentonboarding.NewIncomeSaverBinding(mod.SaveOnboardingIncome),
		CardSaver:        agentonboarding.NewCardSaverBinding(mod.SaveOnboardingCard),
		SplitsSaver:      agentonboarding.NewSplitsSaverBinding(mod.SaveOnboardingBudgetSplits),
		PhaseSetter:      agentonboarding.NewPhaseSetterBinding(mod.SetOnboardingPhase),
		ContextLoader:    agentonboarding.NewContextLoaderBinding(mod.GetOnboardingContext),
		SessionCompleter: agentonboarding.NewSessionCompleterBinding(mod.CompleteOnboardingSession),
		HistoryGateway:   agentonboarding.NewOnboardingHistoryGateway(mod.AppendOnboardingTurn, mod.LoadOnboardingTurns, mod.MarkWelcomeSent),
		O11y:             o11y,
	}
	def := agentwf.BuildOnboardingDefinition(deps)
	store := wfpostgres.NewStoreFactory(o11y).Store(db)
	engine := platform.NewEngine[agentwf.OnboardingState](store, o11y)
	checker := agentonboarding.NewOnboardingProgressChecker(mod.GetOnboardingContext)
	routed := o11y.Metrics().Counter("agent_intent_routed_total", "", "1")
	agent := appservices.NewOnboardingAgent(o11y, routed, engine, def, store, checker, deps.HistoryGateway)

	waNumber := fmt.Sprintf("+55119%08d", time.Now().UnixNano()%100000000)
	userID := SeedActiveUserWA(t, db, waNumber)
	_, err = mod.StartBudgetConfiguration.Execute(ctx, onbusecases.StartBudgetConfigurationInput{
		UserID:  userID,
		Channel: onbentities.OnboardingChannelWhatsApp,
	})
	require.NoError(t, err)

	turns := []struct{ text, why string }{
		{"oi", "ETAPA 1 boas-vindas"},
		{"vamos lá", "ETAPA 1 handshake"},
		{"quero quitar minhas dívidas", "ETAPA 2 objetivo (linguagem natural)"},
		{"recebo uns 5 mil por mês", "ETAPA 3 orçamento (valor por extenso)"},
		{"tenho o Nubank que vence dia 15", "ETAPA 4 cartão (apelido+vencimento)"},
		{"não, só esse mesmo", "ETAPA 4 encerra cartões"},
		{"faz sentido sim", "ETAPA 5 confirma categorias"},
		{"2 mil", "ETAPA 6 Custo Fixo"},
		{"500", "ETAPA 6 Conhecimento"},
		{"500", "ETAPA 6 Prazeres"},
		{"1000", "ETAPA 6 Metas"},
		{"1000", "ETAPA 6 Liberdade Financeira"},
		{"está tudo certo", "ETAPA 7 confirma resumo -> ETAPA 8"},
	}

	t.Log("==================== JORNADA REAL DE ONBOARDING (WhatsApp -> agent -> OpenRouter -> Postgres) ====================")
	for i, turn := range turns {
		wamid := fmt.Sprintf("wamid.onb.%d", i+1)
		res, ok := agent.Handle(ctx, userID, "whatsapp", waNumber, turn.text, wamid)
		require.True(t, ok, "turno %d (%s) não foi tratado", i+1, turn.why)
		require.NotEmpty(t, res.Reply, "turno %d (%s) sem resposta", i+1, turn.why)
		t.Logf("\n[%02d] %s\n👤 %s\n🤖 %s", i+1, turn.why, turn.text, res.Reply)
	}

	t.Log("\n==================== EVIDÊNCIA DE PERSISTÊNCIA (Postgres real) ====================")

	var sessionState string
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT state FROM mecontrola.onboarding_sessions WHERE user_id=$1`, userID).Scan(&sessionState))
	require.Equal(t, "active", sessionState, "sessão deve concluir (state=active)")

	var objective string
	var incomeCents int64
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT payload->>'objective', (payload->>'income_cents')::bigint FROM mecontrola.onboarding_sessions WHERE user_id=$1`,
		userID).Scan(&objective, &incomeCents))
	require.NotEmpty(t, objective, "objetivo deve persistir")
	require.Equal(t, int64(500000), incomeCents, "renda deve persistir em centavos")

	var completedCount int
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT count(*) FROM mecontrola.outbox_events WHERE event_type='onboarding.completed' AND aggregate_user_id=$1`,
		userID).Scan(&completedCount))
	require.Equal(t, 1, completedCount, "1 evento onboarding.completed")

	dispatchOnboardingOutbox(t, db, "onboarding.card_registered", userID, cardConsumer)
	dispatchOnboardingOutbox(t, db, "onboarding.card_registered", userID, cardConsumer)
	var cardCount, dueDay int
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT count(*), coalesce(max(due_day),0) FROM mecontrola.cards WHERE user_id=$1 AND deleted_at IS NULL`,
		userID).Scan(&cardCount, &dueDay))
	require.Equal(t, 1, cardCount, "1 cartão propagado (idempotente)")
	require.Equal(t, 15, dueDay, "vencimento dia 15")

	dispatchOnboardingOutbox(t, db, "onboarding.splits_calculated", userID, budgetsModule.OnboardingBudgetConsumer)
	dispatchOnboardingOutbox(t, db, "onboarding.splits_calculated", userID, budgetsModule.OnboardingBudgetConsumer)
	var budgetCount int
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT count(*) FROM mecontrola.budgets WHERE user_id=$1`, userID).Scan(&budgetCount))
	require.Equal(t, 1, budgetCount, "1 orçamento ativo propagado (idempotente)")

	dumpRows(t, db, "(K) workflow_runs (onboarding)",
		`SELECT workflow, status FROM mecontrola.workflow_runs WHERE correlation_key=$1`, userID.String())
	dumpRows(t, db, "(P) outbox_events (onboarding)",
		`SELECT event_type FROM mecontrola.outbox_events WHERE aggregate_user_id=$1 ORDER BY occurred_at`, userID)

	t.Log("\n✅ Jornada real concluída: 8 etapas via LLM real, persistida em onboarding_sessions + workflow_runs + outbox, propagada para cards e budgets")
}

func dispatchOnboardingOutbox(t *testing.T, db *sqlx.DB, eventType string, userID uuid.UUID, handler platformevents.Handler) {
	t.Helper()
	var (
		id      string
		payload []byte
		aggUser string
	)
	err := db.QueryRowContext(context.Background(),
		`SELECT id, payload, aggregate_user_id FROM mecontrola.outbox_events WHERE event_type=$1 AND aggregate_user_id=$2 ORDER BY created_at DESC LIMIT 1`,
		eventType, userID.String()).Scan(&id, &payload, &aggUser)
	require.NoError(t, err, "evento %s não encontrado no outbox", eventType)
	env := outbox.Envelope{ID: id, EventType: eventType, AggregateUserID: aggUser, Payload: payload}
	require.NoError(t, handler.Handle(context.Background(), platformevents.Event(&onboardingStubEvent{eventType: eventType, payload: env})))
}

type onboardingStubEvent struct {
	eventType string
	payload   any
}

func (e *onboardingStubEvent) GetEventType() string { return e.eventType }
func (e *onboardingStubEvent) GetPayload() any      { return e.payload }
