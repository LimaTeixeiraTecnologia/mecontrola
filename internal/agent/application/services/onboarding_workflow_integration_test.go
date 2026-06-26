//go:build integration

package services

import (
	"context"
	"database/sql"
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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	agentwf "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets"
	cardusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/usecases"
	cardconsumers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/infrastructure/messaging/database/consumers"
	cardrepo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding"
	onbinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/input"
	onbusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	onbentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	onbvalueobjects "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
	platformevents "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/idempotency"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
	wfpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow/infrastructure/postgres"
)

const e2eCardClosingOffsetDays = 10

type deterministicInterpreter struct{}

func (d *deterministicInterpreter) RenderWelcome(_ context.Context) string {
	return "Bem-vindo! Vamos começar?"
}

func (d *deterministicInterpreter) RenderObjective(_ context.Context) string {
	return "Qual seu objetivo?"
}

func (d *deterministicInterpreter) RenderBudget(_ context.Context) string {
	return "Quanto voce recebe?"
}

func (d *deterministicInterpreter) RenderCards(_ context.Context, loop int) string {
	if loop == 0 {
		return "Usa cartao?"
	}
	return "Outro cartao?"
}

func (d *deterministicInterpreter) RenderCategories(_ context.Context) string {
	return "5 categorias. Faz sentido?"
}

func (d *deterministicInterpreter) RenderValues(_ context.Context, pending string) string {
	return fmt.Sprintf("Quanto para %s?", pending)
}

func (d *deterministicInterpreter) RenderSummary(_ context.Context, state agentwf.SummaryState) string {
	return fmt.Sprintf("Resumo: %s com renda %d. Esta tudo certo?", state.Objective, state.IncomeCents)
}

func (d *deterministicInterpreter) RenderRetry(_ context.Context, phase string) string {
	return fmt.Sprintf("Nao entendi (%s). Repita.", phase)
}

func (d *deterministicInterpreter) RenderDailyRedirect(_ context.Context, phase string) string {
	return fmt.Sprintf("Termine o setup primeiro (%s).", phase)
}

func (d *deterministicInterpreter) RenderConclusion(_ context.Context) string {
	return "Pronto!"
}

func (d *deterministicInterpreter) ParseObjective(_ context.Context, text string) (agentwf.ParsedObjective, error) {
	trimmed := strings.TrimSpace(text)
	if isDailyCommandTextIntegration(trimmed) {
		return agentwf.ParsedObjective{DailyCommand: true}, nil
	}
	return agentwf.ParsedObjective{Objective: trimmed, Ambiguous: trimmed == ""}, nil
}

func (d *deterministicInterpreter) ParseBudget(_ context.Context, text string) (agentwf.ParsedBudget, error) {
	trimmed := strings.TrimSpace(text)
	if isDailyCommandTextIntegration(trimmed) {
		return agentwf.ParsedBudget{DailyCommand: true}, nil
	}
	cents, ok := parseMoneyCentsIntegration(trimmed)
	if !ok {
		return agentwf.ParsedBudget{Ambiguous: true}, nil
	}
	return agentwf.ParsedBudget{IncomeCents: cents}, nil
}

func (d *deterministicInterpreter) ParseCards(_ context.Context, text string, loop int) (agentwf.ParsedCards, error) {
	trimmed := strings.TrimSpace(text)
	lower := strings.ToLower(trimmed)
	if isDailyCommandTextIntegration(trimmed) {
		return agentwf.ParsedCards{DailyCommand: true}, nil
	}
	if isNegationIntegration(lower) {
		return agentwf.ParsedCards{Skip: true}, nil
	}
	if loop == 0 && isConfirmationIntegration(lower) {
		return agentwf.ParsedCards{AddAnother: true}, nil
	}
	nickname, dueDay, ok := extractCardIntegration(trimmed)
	if !ok {
		return agentwf.ParsedCards{Ambiguous: true}, nil
	}
	return agentwf.ParsedCards{Nickname: nickname, DueDay: dueDay}, nil
}

func (d *deterministicInterpreter) ParseCategoriesConfirm(_ context.Context, _ string) (bool, error) {
	return true, nil
}

func (d *deterministicInterpreter) ParseValue(_ context.Context, text string) (int64, bool, error) {
	trimmed := strings.TrimSpace(text)
	cents, ok := parseMoneyCentsIntegration(trimmed)
	if !ok {
		return 0, true, nil
	}
	return cents, false, nil
}

func (d *deterministicInterpreter) ParseSummary(_ context.Context, text string) (agentwf.ParsedSummary, error) {
	trimmed := strings.TrimSpace(text)
	lower := strings.ToLower(trimmed)
	if isDailyCommandTextIntegration(trimmed) {
		return agentwf.ParsedSummary{DailyCommand: true}, nil
	}
	if isConfirmationIntegration(lower) {
		return agentwf.ParsedSummary{Confirm: true}, nil
	}
	if isNegationIntegration(lower) || strings.Contains(lower, "corrigir") || strings.Contains(lower, "errado") {
		return parseCorrectionIntegration(trimmed), nil
	}
	return agentwf.ParsedSummary{}, nil
}

func isDailyCommandTextIntegration(text string) bool {
	lower := strings.ToLower(text)
	cues := []string{
		"gastei", "gasto", "comprei", "paguei", "recebi", "salário", "salario",
		"cartão", "cartao", "fatura", "como estou", "resumo", "orçamento", "orcamento",
	}
	for _, cue := range cues {
		if strings.Contains(lower, cue) {
			return true
		}
	}
	return false
}

func isConfirmationIntegration(text string) bool {
	switch text {
	case "sim", "yes", "vamos", "começar", "bora", "ok", "certo", "quero", "acho que sim":
		return true
	default:
		return false
	}
}

func isNegationIntegration(text string) bool {
	switch text {
	case "não", "nao", "no", "nunca", "não uso", "nao uso", "não tenho", "nao tenho":
		return true
	default:
		return false
	}
}

func parseMoneyCentsIntegration(text string) (int64, bool) {
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

var cardDayRe = regexp.MustCompile(`(\d{1,2})`)

func extractCardIntegration(text string) (string, int, bool) {
	trimmed := strings.TrimSpace(text)
	matches := cardDayRe.FindStringSubmatch(trimmed)
	if len(matches) < 2 {
		return "", 0, false
	}
	day, err := strconv.Atoi(matches[1])
	if err != nil || day < 1 || day > 31 {
		return "", 0, false
	}
	nickname := strings.TrimSpace(cardDayRe.ReplaceAllString(trimmed, ""))
	if nickname == "" {
		return "", 0, false
	}
	return nickname, day, true
}

func parseCorrectionIntegration(text string) agentwf.ParsedSummary {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "orcamento"), strings.Contains(lower, "renda"):
		return agentwf.ParsedSummary{Correct: true, Target: agentwf.CorrectionTargetBudget, NewValue: extractMoneyTextIntegration(text)}
	case strings.Contains(lower, "objetivo"):
		return agentwf.ParsedSummary{Correct: true, Target: agentwf.CorrectionTargetObjective, NewValue: extractObjectiveTextIntegration(text)}
	default:
		return agentwf.ParsedSummary{Ambiguous: true}
	}
}

var moneyRe = regexp.MustCompile(`[Rr]\$\s*[\d.]+,?\d*`)
var simpleNumberRe = regexp.MustCompile(`\d+[,.]?\d*`)

func extractMoneyTextIntegration(text string) string {
	if match := moneyRe.FindString(text); match != "" {
		return match
	}
	if match := simpleNumberRe.FindString(text); match != "" {
		return match
	}
	return text
}

func extractObjectiveTextIntegration(text string) string {
	lower := strings.ToLower(text)
	idx := strings.Index(lower, "objetivo")
	if idx >= 0 {
		start := idx + len("objetivo")
		if start < len(text) {
			value := strings.TrimSpace(text[start:])
			value = strings.TrimPrefix(value, "para")
			value = strings.TrimPrefix(value, "eh")
			value = strings.TrimPrefix(value, "é")
			return strings.TrimSpace(value)
		}
	}
	return text
}

type integrationWelcomeMarker struct {
	uc *onbusecases.MarkWelcomeSent
}

func (b *integrationWelcomeMarker) Mark(ctx context.Context, userID uuid.UUID) (bool, error) {
	out, err := b.uc.Execute(ctx, onbusecases.MarkWelcomeSentInput{UserID: userID})
	if err != nil {
		return false, err
	}
	return out.AlreadySent, nil
}

type integrationObjectiveSaver struct {
	uc *onbusecases.SaveOnboardingObjective
}

func (b *integrationObjectiveSaver) Save(ctx context.Context, userID uuid.UUID, objective string) error {
	_, err := b.uc.Execute(ctx, onbusecases.SaveOnboardingObjectiveInput{UserID: userID, Objective: objective})
	return err
}

type integrationIncomeSaver struct {
	uc *onbusecases.SaveOnboardingIncome
}

func (b *integrationIncomeSaver) Save(ctx context.Context, userID uuid.UUID, incomeCents int64) error {
	_, err := b.uc.Execute(ctx, onbusecases.SaveOnboardingIncomeInput{UserID: userID, IncomeCents: incomeCents})
	return err
}

type integrationCardSaver struct {
	uc *onbusecases.SaveOnboardingCard
}

func (b *integrationCardSaver) Save(ctx context.Context, userID uuid.UUID, nickname string, dueDay int) error {
	_, err := b.uc.Execute(ctx, onbinput.SaveOnboardingCardInput{UserID: userID, Nickname: nickname, DueDay: dueDay})
	return err
}

type integrationSplitsSaver struct {
	uc *onbusecases.SaveOnboardingBudgetSplits
}

func (b *integrationSplitsSaver) Save(ctx context.Context, userID uuid.UUID, values map[string]int64) (bool, error) {
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

type integrationPhaseSetter struct {
	uc *onbusecases.SetOnboardingPhase
}

func (b *integrationPhaseSetter) Set(ctx context.Context, userID uuid.UUID, phase string) error {
	p, err := onbvalueobjects.ParseOnboardingPhase(phase)
	if err != nil {
		return err
	}
	_, err = b.uc.Execute(ctx, onbusecases.SetOnboardingPhaseInput{UserID: userID, Phase: p})
	return err
}

type integrationSessionCompleter struct {
	uc *onbusecases.CompleteOnboardingSession
}

func (b *integrationSessionCompleter) Complete(ctx context.Context, userID uuid.UUID) error {
	_, err := b.uc.Execute(ctx, onbusecases.CompleteOnboardingSessionInput{UserID: userID})
	return err
}

type integrationContextLoader struct {
	uc *onbusecases.GetOnboardingContext
}

func (b *integrationContextLoader) Load(ctx context.Context, userID uuid.UUID) (agentwf.OnboardingContext, error) {
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

type integrationStateChecker struct {
	uc *onbusecases.GetOnboardingContext
}

func (c *integrationStateChecker) Check(ctx context.Context, userID uuid.UUID) (bool, onbvalueobjects.OnboardingPhase, error) {
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

type onboardingWorkflowIntegrationSuite struct {
	suite.Suite
	ctx              context.Context
	db               *sqlx.DB
	o11y             *noop.Provider
	storeFactory     platform.StoreFactory
	onboardingCfg    configs.OnboardingConfig
	onboardingModule onboarding.OnboardingModule
	identityModule   identity.IdentityModule
	categoriesModule *categories.CategoriesModule
	budgetsModule    *budgets.BudgetsModule
	cardConsumer     platformevents.Handler
}

func TestOnboardingWorkflowIntegrationSuite(t *testing.T) {
	suite.Run(t, new(onboardingWorkflowIntegrationSuite))
}

func (s *onboardingWorkflowIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()
	s.db, _ = testcontainer.Postgres(s.T())
	s.o11y = noop.NewProvider()
	s.storeFactory = wfpostgres.NewStoreFactory(s.o11y)

	cfg := &configs.Config{}
	identityModule, err := identity.NewIdentityModule(cfg, s.o11y, s.db)
	s.Require().NoError(err)
	s.identityModule = identityModule

	gatewayAuth := func(h http.Handler) http.Handler { return h }
	catModule := categories.NewCategoriesModule(s.db, s.o11y, gatewayAuth)
	s.categoriesModule = catModule

	budgetsModule, err := budgets.NewBudgetsModule(cfg, s.o11y, s.db, catModule, gatewayAuth, nil, nil)
	s.Require().NoError(err)
	s.budgetsModule = budgetsModule

	cardFactory := cardrepo.NewRepositoryFactory(s.o11y)
	cardIdemStorage := idempotency.NewPostgresStorage(s.db)
	createCardUoW := uow.NewUnitOfWork(s.db)
	createCard := cardusecases.NewCreateCard(createCardUoW, cardFactory, cardIdemStorage, s.o11y)
	s.cardConsumer = cardconsumers.NewOnboardingCardConsumer(createCard, cardIdemStorage, s.o11y)

	onboardingCfg := configs.OnboardingConfig{
		TokenEncryptionKey:     "0123456789abcdef0123456789abcdef",
		CardClosingOffsetDays:  e2eCardClosingOffsetDays,
		AbandonmentTTLHours:    48,
		AbandonmentJobSchedule: "@hourly",
		AbandonmentBatchSize:   100,
	}
	s.onboardingCfg = onboardingCfg
	s.onboardingModule = s.newOnboardingModule(&synchronousCardCreator{db: s.db})
}

func (s *onboardingWorkflowIntegrationSuite) newOnboardingModule(cardCreator onbusecases.SynchronousCardCreator) onboarding.OnboardingModule {
	mod, err := onboarding.NewOnboardingModule(
		s.db,
		s.onboardingCfg,
		configs.WhatsAppConfig{PhoneNumberID: "123456789", AccessToken: "fake-token"},
		configs.OutboxConfig{RetryMaxAttempts: 3},
		configs.EmailConfig{Provider: "smtp", SMTPHost: "smtp.example.com", SMTPPort: 587, FromAddress: "test@mecontrola.test", FromName: "Test"},
		s.identityModule,
		s.o11y,
		onboarding.WithOnboardingCardCreator(cardCreator),
	)
	s.Require().NoError(err)
	return mod
}

func (s *onboardingWorkflowIntegrationSuite) insertUser() uuid.UUID {
	userID := uuid.New()
	number := fmt.Sprintf("+5511%09d", time.Now().UnixNano()%1000000000)
	_, err := s.db.ExecContext(s.ctx,
		`INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at) VALUES ($1, $2, 'ACTIVE', now(), now())`,
		userID, number,
	)
	s.Require().NoError(err)
	return userID
}

func (s *onboardingWorkflowIntegrationSuite) startSession(userID uuid.UUID) {
	_, err := s.onboardingModule.StartBudgetConfiguration.Execute(s.ctx, onbusecases.StartBudgetConfigurationInput{
		UserID:  userID,
		Channel: onbentities.OnboardingChannelWhatsApp,
	})
	s.Require().NoError(err)
}

func (s *onboardingWorkflowIntegrationSuite) newAgent() *OnboardingAgent {
	return s.newAgentFromModule(s.onboardingModule)
}

func (s *onboardingWorkflowIntegrationSuite) newAgentFromModule(uc onboarding.OnboardingModule) *OnboardingAgent {
	deps := agentwf.OnboardingDeps{
		Interpreter:      &deterministicInterpreter{},
		WelcomeMarker:    &integrationWelcomeMarker{uc: uc.MarkWelcomeSent},
		ObjectiveSaver:   &integrationObjectiveSaver{uc: uc.SaveOnboardingObjective},
		IncomeSaver:      &integrationIncomeSaver{uc: uc.SaveOnboardingIncome},
		CardSaver:        &integrationCardSaver{uc: uc.SaveOnboardingCard},
		SplitsSaver:      &integrationSplitsSaver{uc: uc.SaveOnboardingBudgetSplits},
		PhaseSetter:      &integrationPhaseSetter{uc: uc.SetOnboardingPhase},
		ContextLoader:    &integrationContextLoader{uc: uc.GetOnboardingContext},
		SessionCompleter: &integrationSessionCompleter{uc: uc.CompleteOnboardingSession},
		O11y:             s.o11y,
	}
	def := agentwf.BuildOnboardingDefinition(deps)
	store := s.storeFactory.Store(s.db)
	engine := platform.NewEngine[agentwf.OnboardingState](store, s.o11y)
	checker := &integrationStateChecker{uc: uc.GetOnboardingContext}
	routedTotal := s.o11y.Metrics().Counter("agent_intent_routed_total", "", "1")
	return NewOnboardingAgent(s.o11y, routedTotal, engine, def, store, checker, nil)
}

func (s *onboardingWorkflowIntegrationSuite) TestResumeDurableAcrossTurnsAndEngineRestart() {
	userID := s.insertUser()
	s.startSession(userID)
	checker := &integrationStateChecker{uc: s.onboardingModule.GetOnboardingContext}
	inProgress, err := checker.IsOnboardingInProgress(s.ctx, userID)
	s.Require().NoError(err)
	s.True(inProgress, "session should be in progress")

	agent1 := s.newAgent()

	res, ok := agent1.Handle(s.ctx, userID, "whatsapp", "+5511999999999", "oi", "m1")
	s.True(ok)
	s.Contains(res.Reply, "Bem-vindo")

	res, ok = agent1.Handle(s.ctx, userID, "whatsapp", "+5511999999999", "sim", "m2")
	s.True(ok)
	s.Contains(res.Reply, "objetivo")

	res, ok = agent1.Handle(s.ctx, userID, "whatsapp", "+5511999999999", "viajar", "m3")
	s.True(ok)
	s.Contains(res.Reply, "recebe")

	agent2 := s.newAgent()
	res, ok = agent2.Handle(s.ctx, userID, "whatsapp", "+5511999999999", "4000", "m4")
	s.True(ok)
	s.Contains(res.Reply, "cartao")

	var state string
	err = s.db.QueryRowContext(s.ctx, `SELECT state FROM mecontrola.onboarding_sessions WHERE user_id = $1`, userID).Scan(&state)
	s.Require().NoError(err)
	s.Equal("in_progress", state)
}

func (s *onboardingWorkflowIntegrationSuite) TestReplaySameMessageIDReturnsReplayOutcome() {
	userID := s.insertUser()
	s.startSession(userID)
	agent := s.newAgent()

	_, ok := agent.Handle(s.ctx, userID, "whatsapp", "+5511999999999", "oi", "m1")
	s.True(ok)

	res, ok := agent.Handle(s.ctx, userID, "whatsapp", "+5511999999999", "oi", "m1")
	s.True(ok)
	s.Equal(tools.OutcomeReplay, res.Outcome)
}

func (s *onboardingWorkflowIntegrationSuite) TestHappyPathCompletesAllEightPhases() {
	userID := s.insertUser()
	s.startSession(userID)
	agent := s.newAgent()

	replies := map[string]string{
		"m1":  "oi",
		"m2":  "sim",
		"m3":  "quitar dividas",
		"m4":  "5000",
		"m5":  "Nubank 15",
		"m6":  "nao",
		"m7":  "ok",
		"m8":  "2000",
		"m9":  "500",
		"m10": "500",
		"m11": "1000",
		"m12": "1000",
		"m13": "sim",
	}
	expectedPhases := []string{
		"Bem-vindo",
		"objetivo",
		"recebe",
		"cartao",
		"Outro",
		"categorias",
		"fixed_cost",
		"knowledge",
		"pleasures",
		"goals",
		"financial_freedom",
		"Resumo",
		"Pronto",
	}

	i := 0
	for _, msgID := range []string{"m1", "m2", "m3", "m4", "m5", "m6", "m7", "m8", "m9", "m10", "m11", "m12", "m13"} {
		res, ok := agent.Handle(s.ctx, userID, "whatsapp", "+5511999999999", replies[msgID], msgID)
		s.True(ok, "message %s should be handled", msgID)
		s.Contains(res.Reply, expectedPhases[i], "message %s", msgID)
		i++
	}

	var state string
	err := s.db.QueryRowContext(s.ctx, `SELECT state FROM mecontrola.onboarding_sessions WHERE user_id = $1`, userID).Scan(&state)
	s.Require().NoError(err)
	s.Equal("active", state)

	var completedAt sql.NullTime
	err = s.db.QueryRowContext(s.ctx, `SELECT (payload->>'completed_at')::timestamptz FROM mecontrola.onboarding_sessions WHERE user_id = $1`, userID).Scan(&completedAt)
	s.Require().NoError(err)
	s.True(completedAt.Valid)

	var completedCount int
	err = s.db.QueryRowContext(s.ctx, `SELECT COUNT(*) FROM mecontrola.outbox_events WHERE event_type = 'onboarding.completed' AND aggregate_user_id = $1`, userID).Scan(&completedCount)
	s.Require().NoError(err)
	s.Equal(1, completedCount)
}

func (s *onboardingWorkflowIntegrationSuite) TestCardRegistrationPropagatesToCardModule() {
	userID := s.insertUser()
	mod := s.newOnboardingModule(nil)
	_, err := mod.StartBudgetConfiguration.Execute(s.ctx, onbusecases.StartBudgetConfigurationInput{
		UserID:  userID,
		Channel: onbentities.OnboardingChannelWhatsApp,
	})
	s.Require().NoError(err)

	agent := s.newAgentFromModule(mod)

	agent.Handle(s.ctx, userID, "whatsapp", "+5511999999999", "oi", "m1")
	agent.Handle(s.ctx, userID, "whatsapp", "+5511999999999", "sim", "m2")
	agent.Handle(s.ctx, userID, "whatsapp", "+5511999999999", "viajar", "m3")
	agent.Handle(s.ctx, userID, "whatsapp", "+5511999999999", "5000", "m4")
	agent.Handle(s.ctx, userID, "whatsapp", "+5511999999999", "Nubank 15", "m5")

	var outboxID string
	err = s.db.QueryRowContext(s.ctx,
		`SELECT id FROM mecontrola.outbox_events WHERE event_type = 'onboarding.card_registered' AND aggregate_user_id = $1 ORDER BY created_at DESC LIMIT 1`,
		userID.String(),
	).Scan(&outboxID)
	s.Require().NoError(err)

	err = s.dispatchOutboxEvent(outboxID, s.cardConsumer)
	s.Require().NoError(err)

	err = s.dispatchOutboxEvent(outboxID, s.cardConsumer)
	s.Require().NoError(err)

	var count int
	err = s.db.QueryRowContext(s.ctx, `SELECT COUNT(*) FROM mecontrola.cards WHERE user_id = $1 AND deleted_at IS NULL`, userID).Scan(&count)
	s.Require().NoError(err)
	s.Equal(1, count)

	var closingDay, dueDay int
	err = s.db.QueryRowContext(s.ctx, `SELECT closing_day, due_day FROM mecontrola.cards WHERE user_id = $1 AND deleted_at IS NULL`, userID).Scan(&closingDay, &dueDay)
	s.Require().NoError(err)
	s.Equal(15-e2eCardClosingOffsetDays, closingDay)
	s.Equal(15, dueDay)
}

func (s *onboardingWorkflowIntegrationSuite) TestBudgetSplitsPropagatesToBudgetsModule() {
	userID := s.insertUser()
	s.startSession(userID)
	agent := s.newAgent()

	agent.Handle(s.ctx, userID, "whatsapp", "+5511999999999", "oi", "m1")
	agent.Handle(s.ctx, userID, "whatsapp", "+5511999999999", "sim", "m2")
	agent.Handle(s.ctx, userID, "whatsapp", "+5511999999999", "viajar", "m3")
	agent.Handle(s.ctx, userID, "whatsapp", "+5511999999999", "5000", "m4")
	agent.Handle(s.ctx, userID, "whatsapp", "+5511999999999", "nao", "m5")
	agent.Handle(s.ctx, userID, "whatsapp", "+5511999999999", "ok", "m6")
	agent.Handle(s.ctx, userID, "whatsapp", "+5511999999999", "2000", "m7")
	agent.Handle(s.ctx, userID, "whatsapp", "+5511999999999", "500", "m8")
	agent.Handle(s.ctx, userID, "whatsapp", "+5511999999999", "500", "m9")
	agent.Handle(s.ctx, userID, "whatsapp", "+5511999999999", "1000", "m10")
	agent.Handle(s.ctx, userID, "whatsapp", "+5511999999999", "1000", "m11")

	var outboxID string
	err := s.db.QueryRowContext(s.ctx,
		`SELECT id FROM mecontrola.outbox_events WHERE event_type = 'onboarding.splits_calculated' AND aggregate_user_id = $1 ORDER BY created_at DESC LIMIT 1`,
		userID.String(),
	).Scan(&outboxID)
	s.Require().NoError(err)

	s.dispatchOutboxEvent(outboxID, s.budgetsModule.OnboardingBudgetConsumer)
	s.dispatchOutboxEvent(outboxID, s.budgetsModule.OnboardingBudgetConsumer)

	var count int
	err = s.db.QueryRowContext(s.ctx, `SELECT COUNT(*) FROM mecontrola.budgets WHERE user_id = $1`, userID).Scan(&count)
	s.Require().NoError(err)
	s.Equal(1, count)
}

func (s *onboardingWorkflowIntegrationSuite) TestDailyCommandDuringOnboardingRedirectsWithoutPersistingTransaction() {
	userID := s.insertUser()
	s.startSession(userID)
	agent := s.newAgent()

	agent.Handle(s.ctx, userID, "whatsapp", "+5511999999999", "oi", "m1")
	res, ok := agent.Handle(s.ctx, userID, "whatsapp", "+5511999999999", "gastei 50 mercado", "m2")
	s.True(ok)
	s.Contains(res.Reply, "Termine")

	var objective sql.NullString
	err := s.db.QueryRowContext(s.ctx, `SELECT payload->>'objective' FROM mecontrola.onboarding_sessions WHERE user_id = $1`, userID).Scan(&objective)
	s.Require().NoError(err)
	s.False(objective.Valid)
}

func (s *onboardingWorkflowIntegrationSuite) TestSummaryCorrectionUpdatesObjective() {
	userID := s.insertUser()
	s.startSession(userID)
	agent := s.newAgent()

	agent.Handle(s.ctx, userID, "whatsapp", "+5511999999999", "oi", "m1")
	agent.Handle(s.ctx, userID, "whatsapp", "+5511999999999", "sim", "m2")
	agent.Handle(s.ctx, userID, "whatsapp", "+5511999999999", "viajar", "m3")
	agent.Handle(s.ctx, userID, "whatsapp", "+5511999999999", "5000", "m4")
	agent.Handle(s.ctx, userID, "whatsapp", "+5511999999999", "nao", "m5")
	agent.Handle(s.ctx, userID, "whatsapp", "+5511999999999", "ok", "m6")
	agent.Handle(s.ctx, userID, "whatsapp", "+5511999999999", "2000", "m7")
	agent.Handle(s.ctx, userID, "whatsapp", "+5511999999999", "500", "m8")
	agent.Handle(s.ctx, userID, "whatsapp", "+5511999999999", "500", "m9")
	agent.Handle(s.ctx, userID, "whatsapp", "+5511999999999", "1000", "m10")
	agent.Handle(s.ctx, userID, "whatsapp", "+5511999999999", "1000", "m11")

	res, ok := agent.Handle(s.ctx, userID, "whatsapp", "+5511999999999", "corrigir objetivo para comprar casa", "m12")
	s.True(ok)
	s.Contains(res.Reply, "comprar casa")

	var objective string
	err := s.db.QueryRowContext(s.ctx, `SELECT payload->>'objective' FROM mecontrola.onboarding_sessions WHERE user_id = $1`, userID).Scan(&objective)
	s.Require().NoError(err)
	s.Equal("comprar casa", objective)
}

func (s *onboardingWorkflowIntegrationSuite) TestLegacyPhaseMigrationResetsToWelcome() {
	userID := s.insertUser()
	_, err := s.db.ExecContext(s.ctx,
		`INSERT INTO mecontrola.onboarding_sessions (user_id, channel, state, payload, updated_at) VALUES ($1, 'whatsapp', 'in_progress', $2, now())`,
		userID,
		`{"phase":"first_tx","objective":"","income_cents":0}`,
	)
	s.Require().NoError(err)

	agent := s.newAgent()
	res, ok := agent.Handle(s.ctx, userID, "whatsapp", "+5511999999999", "sim", "m1")
	s.True(ok)
	s.Contains(res.Reply, "Bem-vindo")

	var phase string
	err = s.db.QueryRowContext(s.ctx, `SELECT payload->>'phase' FROM mecontrola.onboarding_sessions WHERE user_id = $1`, userID).Scan(&phase)
	s.Require().NoError(err)
	s.Equal("welcome", phase)
}

func (s *onboardingWorkflowIntegrationSuite) dispatchOutboxEvent(eventID string, handler platformevents.Handler) error {
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
	return handler.Handle(s.ctx, platformevents.Event(&testStubEvent{eventType: eventType, payload: env}))
}

type testStubEvent struct {
	eventType string
	payload   any
}

func (e *testStubEvent) GetEventType() string { return e.eventType }
func (e *testStubEvent) GetPayload() any      { return e.payload }

type synchronousCardCreator struct {
	db *sqlx.DB
}

func (c *synchronousCardCreator) Execute(ctx context.Context, userID, nickname string, closingDay int) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return err
	}
	_, err = c.db.ExecContext(ctx,
		`INSERT INTO mecontrola.cards (id, user_id, name, nickname, closing_day, due_day, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, now(), now())`,
		uuid.New(), uid, nickname, nickname, closingDay, 1,
	)
	return err
}

func (s *onboardingWorkflowIntegrationSuite) TearDownTest() {
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
