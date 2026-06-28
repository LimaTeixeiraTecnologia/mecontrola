//go:build integration

package consumers_test

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	agentservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	agentusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	agentwf "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/messaging/database/consumers"
	agentrepo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding"
	onbinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/input"
	onbusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	onbentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	onbvalueobjects "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
	platformevents "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
	wfpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow/infrastructure/postgres"
)

const completedCardClosingOffsetDays = 10

type completedInterpreter struct{}

func (d *completedInterpreter) RenderWelcome(_ context.Context) string   { return "Bem-vindo" }
func (d *completedInterpreter) RenderObjective(_ context.Context) string { return "objetivo" }
func (d *completedInterpreter) RenderBudget(_ context.Context) string    { return "recebe" }
func (d *completedInterpreter) RenderCards(_ context.Context, loop int) string {
	if loop == 0 {
		return "cartao"
	}
	return "Outro"
}
func (d *completedInterpreter) RenderCategories(_ context.Context) string { return "categorias" }
func (d *completedInterpreter) RenderValues(_ context.Context, pending string) string {
	return fmt.Sprintf("Quanto para %s?", pending)
}
func (d *completedInterpreter) RenderSummary(_ context.Context, state agentwf.SummaryState) string {
	return fmt.Sprintf("Resumo: %s com renda %d", state.Objective, state.IncomeCents)
}
func (d *completedInterpreter) RenderRetry(_ context.Context, _ string) string { return "Repita" }
func (d *completedInterpreter) RenderDailyRedirect(_ context.Context, _ string) string {
	return "Termine"
}
func (d *completedInterpreter) RenderConclusion(_ context.Context) string { return "Pronto" }
func (d *completedInterpreter) RenderObjectiveSaved(_ context.Context) string {
	return "Perfeito"
}
func (d *completedInterpreter) RenderBudgetSaved(_ context.Context, _ int64) string { return "ok" }
func (d *completedInterpreter) RenderCardSaved(_ context.Context, _ string, _ int) string {
	return "ok"
}
func (d *completedInterpreter) RenderValueSaved(_ context.Context, _ string, _ int64) string {
	return "ok"
}
func (d *completedInterpreter) RenderCategoriesConfirmed(_ context.Context) string { return "ok" }
func (d *completedInterpreter) RenderCategoriesClarify(_ context.Context) string   { return "ok" }
func (d *completedInterpreter) RenderValuesMismatch(_ context.Context, _, _ int64) string {
	return "ok"
}

func (d *completedInterpreter) ParseObjective(_ context.Context, text string) (agentwf.ParsedObjective, error) {
	return agentwf.ParsedObjective{Objective: strings.TrimSpace(text)}, nil
}
func (d *completedInterpreter) ParseBudget(_ context.Context, text string) (agentwf.ParsedBudget, error) {
	cents, ok := parseMoneyCompleted(text)
	if !ok {
		return agentwf.ParsedBudget{Ambiguous: true}, nil
	}
	return agentwf.ParsedBudget{IncomeCents: cents}, nil
}
func (d *completedInterpreter) ParseCards(_ context.Context, text string, _ int) (agentwf.ParsedCards, error) {
	if strings.ToLower(strings.TrimSpace(text)) == "nao" {
		return agentwf.ParsedCards{Skip: true}, nil
	}
	nickname, dueDay, ok := extractCardCompleted(text)
	if !ok {
		return agentwf.ParsedCards{Ambiguous: true}, nil
	}
	return agentwf.ParsedCards{Nickname: nickname, DueDay: dueDay}, nil
}
func (d *completedInterpreter) ParseCategoriesConfirm(_ context.Context, _ string) (bool, error) {
	return true, nil
}
func (d *completedInterpreter) ParseValue(_ context.Context, text string) (agentwf.ParsedValue, error) {
	cents, ok := parseMoneyCompleted(text)
	if !ok {
		return agentwf.ParsedValue{Ambiguous: true}, nil
	}
	return agentwf.ParsedValue{ValueCents: cents}, nil
}
func (d *completedInterpreter) ParseSummary(_ context.Context, text string) (agentwf.ParsedSummary, error) {
	if strings.ToLower(strings.TrimSpace(text)) == "sim" {
		return agentwf.ParsedSummary{Confirm: true}, nil
	}
	return agentwf.ParsedSummary{Confirm: true}, nil
}

var completedCardDayRe = regexp.MustCompile(`(\d{1,2})`)

func extractCardCompleted(text string) (string, int, bool) {
	trimmed := strings.TrimSpace(text)
	matches := completedCardDayRe.FindStringSubmatch(trimmed)
	if len(matches) < 2 {
		return "", 0, false
	}
	day, err := strconv.Atoi(matches[1])
	if err != nil || day < 1 || day > 31 {
		return "", 0, false
	}
	nickname := strings.TrimSpace(completedCardDayRe.ReplaceAllString(trimmed, ""))
	if nickname == "" {
		return "", 0, false
	}
	return nickname, day, true
}

func parseMoneyCompleted(text string) (int64, bool) {
	cleaned := strings.TrimSpace(text)
	cleaned = strings.ReplaceAll(cleaned, "R$", "")
	cleaned = strings.ReplaceAll(cleaned, "r$", "")
	cleaned = strings.ReplaceAll(cleaned, " ", "")
	cleaned = strings.ReplaceAll(cleaned, ".", "")
	cleaned = strings.ReplaceAll(cleaned, ",", ".")
	if cleaned == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(cleaned, 64)
	if err != nil || value < 0 {
		return 0, false
	}
	return int64(value * 100), true
}

type completedWelcomeMarker struct{ uc *onbusecases.MarkWelcomeSent }

func (b *completedWelcomeMarker) Mark(ctx context.Context, userID uuid.UUID) (bool, error) {
	out, err := b.uc.Execute(ctx, onbusecases.MarkWelcomeSentInput{UserID: userID})
	if err != nil {
		return false, err
	}
	return out.AlreadySent, nil
}

type completedObjectiveSaver struct {
	uc *onbusecases.SaveOnboardingObjective
}

func (b *completedObjectiveSaver) Save(ctx context.Context, userID uuid.UUID, objective string) error {
	_, err := b.uc.Execute(ctx, onbusecases.SaveOnboardingObjectiveInput{UserID: userID, Objective: objective})
	return err
}

type completedIncomeSaver struct {
	uc *onbusecases.SaveOnboardingIncome
}

func (b *completedIncomeSaver) Save(ctx context.Context, userID uuid.UUID, incomeCents int64) error {
	_, err := b.uc.Execute(ctx, onbusecases.SaveOnboardingIncomeInput{UserID: userID, IncomeCents: incomeCents})
	return err
}

type completedCardSaver struct {
	uc *onbusecases.SaveOnboardingCard
}

func (b *completedCardSaver) Save(ctx context.Context, userID uuid.UUID, nickname string, dueDay int) error {
	_, err := b.uc.Execute(ctx, onbinput.SaveOnboardingCardInput{UserID: userID, Nickname: nickname, DueDay: dueDay})
	return err
}

type completedSplitsSaver struct {
	uc *onbusecases.SaveOnboardingBudgetSplits
}

func (b *completedSplitsSaver) Save(ctx context.Context, userID uuid.UUID, values map[string]int64) (bool, error) {
	slugToKind := map[string]onbvalueobjects.CategoryKind{
		"fixed_cost":        onbvalueobjects.CategoryKindFixedCost,
		"knowledge":         onbvalueobjects.CategoryKindKnowledge,
		"pleasures":         onbvalueobjects.CategoryKindPleasures,
		"goals":             onbvalueobjects.CategoryKindGoals,
		"financial_freedom": onbvalueobjects.CategoryKindFinancialFreedom,
	}
	items := make([]onbusecases.BudgetSplitItem, 0, len(values))
	for slug, amount := range values {
		kind, ok := slugToKind[slug]
		if !ok {
			continue
		}
		items = append(items, onbusecases.BudgetSplitItem{Kind: kind, AmountCents: amount})
	}
	out, err := b.uc.Execute(ctx, onbusecases.SaveOnboardingBudgetSplitsInput{UserID: userID, Allocations: items})
	if err != nil {
		return false, err
	}
	return out.Applied, nil
}

type completedPhaseSetter struct {
	uc *onbusecases.SetOnboardingPhase
}

func (b *completedPhaseSetter) Set(ctx context.Context, userID uuid.UUID, phase string) error {
	p, err := onbvalueobjects.ParseOnboardingPhase(phase)
	if err != nil {
		return err
	}
	_, err = b.uc.Execute(ctx, onbusecases.SetOnboardingPhaseInput{UserID: userID, Phase: p})
	return err
}

type completedSessionCompleter struct {
	uc *onbusecases.CompleteOnboardingSession
}

func (b *completedSessionCompleter) Complete(ctx context.Context, userID uuid.UUID) error {
	_, err := b.uc.Execute(ctx, onbusecases.CompleteOnboardingSessionInput{UserID: userID})
	return err
}

type completedContextLoader struct {
	uc *onbusecases.GetOnboardingContext
}

func (b *completedContextLoader) Load(ctx context.Context, userID uuid.UUID) (agentwf.OnboardingContext, error) {
	out, err := b.uc.Execute(ctx, onbusecases.GetOnboardingContextInput{UserID: userID})
	if err != nil {
		return agentwf.OnboardingContext{}, err
	}
	cards := make([]agentwf.OnboardingCardState, 0, len(out.Cards))
	for _, c := range out.Cards {
		cards = append(cards, agentwf.OnboardingCardState{Name: c.Name, DueDay: c.DueDay})
	}
	return agentwf.OnboardingContext{
		Objective:   out.Objective,
		IncomeCents: out.IncomeCents,
		Cards:       cards,
	}, nil
}

type completedStateChecker struct {
	uc *onbusecases.GetOnboardingContext
}

func (c *completedStateChecker) Check(ctx context.Context, userID uuid.UUID) (bool, onbvalueobjects.OnboardingPhase, error) {
	out, err := c.uc.Execute(ctx, onbusecases.GetOnboardingContextInput{UserID: userID})
	if err != nil {
		return false, onbvalueobjects.PhaseWelcome, err
	}
	if !out.Found {
		return false, onbvalueobjects.PhaseWelcome, nil
	}
	if out.CompletedAt != nil {
		return false, onbvalueobjects.PhaseWelcome, nil
	}
	if !out.Phase.IsValid() {
		return true, onbvalueobjects.PhaseWelcome, nil
	}
	return true, out.Phase, nil
}

type onboardingCompletedIntegrationSuite struct {
	suite.Suite
	ctx              context.Context
	db               *sqlx.DB
	o11y             *noop.Provider
	onboardingModule onboarding.OnboardingModule
	identityModule   identity.IdentityModule
}

func TestOnboardingCompletedIntegrationSuite(t *testing.T) {
	suite.Run(t, new(onboardingCompletedIntegrationSuite))
}

func (s *onboardingCompletedIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()
	s.db, _ = testcontainer.Postgres(s.T())
	s.o11y = noop.NewProvider()

	cfg := &configs.Config{}
	identityModule, err := identity.NewIdentityModule(cfg, s.o11y, s.db)
	s.Require().NoError(err)
	s.identityModule = identityModule

	gatewayAuth := func(h http.Handler) http.Handler { return h }
	catModule := categories.NewCategoriesModule(s.db, s.o11y, gatewayAuth)
	budgetsModule, err := budgets.NewBudgetsModule(cfg, s.o11y, s.db, catModule, gatewayAuth, nil, nil)
	s.Require().NoError(err)
	_ = budgetsModule

	onboardingCfg := configs.OnboardingConfig{
		TokenEncryptionKey:    "0123456789abcdef0123456789abcdef",
		CardClosingOffsetDays: completedCardClosingOffsetDays,
	}
	s.onboardingModule, err = onboarding.NewOnboardingModule(
		s.db,
		onboardingCfg,
		configs.WhatsAppConfig{PhoneNumberID: "123456789", AccessToken: "fake-token"},
		configs.OutboxConfig{RetryMaxAttempts: 3},
		configs.EmailConfig{Provider: "smtp", SMTPHost: "smtp.example.com", SMTPPort: 587, FromAddress: "test@mecontrola.test", FromName: "Test"},
		s.identityModule,
		s.o11y,
	)
	s.Require().NoError(err)
}

func (s *onboardingCompletedIntegrationSuite) insertUser() uuid.UUID {
	userID := uuid.New()
	number := fmt.Sprintf("+5511%09d", time.Now().UnixNano()%1000000000)
	_, err := s.db.ExecContext(s.ctx,
		`INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at) VALUES ($1, $2, 'ACTIVE', now(), now())`,
		userID, number,
	)
	s.Require().NoError(err)
	return userID
}

func (s *onboardingCompletedIntegrationSuite) startSession(userID uuid.UUID) {
	_, err := s.onboardingModule.StartBudgetConfiguration.Execute(s.ctx, onbusecases.StartBudgetConfigurationInput{
		UserID:  userID,
		Channel: onbentities.OnboardingChannelWhatsApp,
	})
	s.Require().NoError(err)
}

func (s *onboardingCompletedIntegrationSuite) newAgent() *agentservices.OnboardingAgent {
	uc := s.onboardingModule
	deps := agentwf.OnboardingDeps{
		Interpreter:      &completedInterpreter{},
		WelcomeMarker:    &completedWelcomeMarker{uc: uc.MarkWelcomeSent},
		ObjectiveSaver:   &completedObjectiveSaver{uc: uc.SaveOnboardingObjective},
		IncomeSaver:      &completedIncomeSaver{uc: uc.SaveOnboardingIncome},
		CardSaver:        &completedCardSaver{uc: uc.SaveOnboardingCard},
		SplitsSaver:      &completedSplitsSaver{uc: uc.SaveOnboardingBudgetSplits},
		PhaseSetter:      &completedPhaseSetter{uc: uc.SetOnboardingPhase},
		ContextLoader:    &completedContextLoader{uc: uc.GetOnboardingContext},
		SessionCompleter: &completedSessionCompleter{uc: uc.CompleteOnboardingSession},
		O11y:             s.o11y,
	}
	def := agentwf.BuildOnboardingDefinition(deps)
	store := wfpostgres.NewStoreFactory(s.o11y).Store(s.db)
	engine := platform.NewEngine[agentwf.OnboardingState](store, s.o11y)
	checker := &completedStateChecker{uc: uc.GetOnboardingContext}
	routedTotal := s.o11y.Metrics().Counter("agent_intent_routed_total", "", "1")
	return agentservices.NewOnboardingAgent(s.o11y, routedTotal, engine, def, store, checker, nil)
}

func (s *onboardingCompletedIntegrationSuite) newCompletedConsumer() platformevents.Handler {
	uc := agentusecases.NewConsolidateOnboardingWorkingMemory(
		uow.NewUnitOfWork(s.db),
		s.onboardingModule.GetOnboardingContext,
		agentrepo.NewWorkingMemoryRepositoryFactory(s.o11y),
		agentrepo.NewProcessedEventRepositoryFactory(s.o11y),
		s.o11y,
	)
	return consumers.NewOnboardingCompletedConsumer(uc, s.o11y)
}

func (s *onboardingCompletedIntegrationSuite) TestCompletedEventPropagatesToWorkingMemoryIdempotently() {
	userID := s.insertUser()
	s.startSession(userID)
	agent := s.newAgent()

	replies := map[string]string{
		"m1":  "oi",
		"m2":  "sim",
		"m3":  "quitar dividas",
		"m4":  "5000",
		"m5":  "nao",
		"m6":  "ok",
		"m7":  "2000",
		"m8":  "500",
		"m9":  "500",
		"m10": "1000",
		"m11": "1000",
		"m12": "sim",
	}
	for _, msgID := range []string{"m1", "m2", "m3", "m4", "m5", "m6", "m7", "m8", "m9", "m10", "m11", "m12"} {
		_, ok := agent.Handle(s.ctx, userID, "whatsapp", "+5511999999999", replies[msgID], msgID)
		s.True(ok, "message %s should be handled", msgID)
	}

	var outboxID string
	err := s.db.QueryRowContext(s.ctx,
		`SELECT id FROM mecontrola.outbox_events WHERE event_type = 'onboarding.completed' AND aggregate_user_id = $1 ORDER BY created_at DESC LIMIT 1`,
		userID.String(),
	).Scan(&outboxID)
	s.Require().NoError(err)

	completedConsumer := s.newCompletedConsumer()
	err = s.dispatchOutboxEvent(outboxID, completedConsumer)
	s.Require().NoError(err)
	err = s.dispatchOutboxEvent(outboxID, completedConsumer)
	s.Require().NoError(err)

	var content string
	err = s.db.QueryRowContext(s.ctx, `SELECT content FROM mecontrola.agent_working_memory WHERE user_id = $1`, userID).Scan(&content)
	s.Require().NoError(err)
	s.Contains(content, "quitar dividas")
	s.Contains(content, "R$ 5.000,00")
	s.Contains(content, "Custo Fixo")

	var processedCount int
	err = s.db.QueryRowContext(s.ctx, `SELECT COUNT(*) FROM mecontrola.agent_processed_events WHERE event_id = $1`, outboxID).Scan(&processedCount)
	s.Require().NoError(err)
	s.Equal(1, processedCount)
}

func (s *onboardingCompletedIntegrationSuite) dispatchOutboxEvent(eventID string, handler platformevents.Handler) error {
	var (
		payload         []byte
		aggregateUserID string
		eventType       string
	)
	err := s.db.QueryRowContext(s.ctx,
		`SELECT event_type, payload, aggregate_user_id FROM mecontrola.outbox_events WHERE id = $1`,
		eventID,
	).Scan(&eventType, &payload, &aggregateUserID)
	if err != nil {
		return err
	}

	env := outbox.Envelope{
		ID:              eventID,
		EventType:       eventType,
		AggregateUserID: aggregateUserID,
		Payload:         payload,
	}
	return handler.Handle(s.ctx, platformevents.Event(&completedStubEvent{eventType: eventType, payload: env}))
}

type completedStubEvent struct {
	eventType string
	payload   any
}

func (e *completedStubEvent) GetEventType() string { return e.eventType }
func (e *completedStubEvent) GetPayload() any      { return e.payload }

func (s *onboardingCompletedIntegrationSuite) TearDownTest() {
	_, err := s.db.ExecContext(s.ctx, `
		TRUNCATE TABLE
			mecontrola.onboarding_sessions,
			mecontrola.outbox_events,
			mecontrola.cards,
			mecontrola.budgets,
			mecontrola.workflow_runs,
			mecontrola.workflow_steps,
			mecontrola.agent_working_memory,
			mecontrola.agent_processed_events,
			mecontrola.users
		RESTART IDENTITY CASCADE
	`)
	s.Require().NoError(err)
}
