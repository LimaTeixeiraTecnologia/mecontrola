//go:build e2e

package e2e_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	agentbinding "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/binding"
	agentrepo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/repositories"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	appusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	agentonboarding "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/onboarding"
	onbusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	onbentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"

	"github.com/google/uuid"
)

type scriptedInterpreter struct {
	queue []appinterfaces.LLMResponse
}

func (s *scriptedInterpreter) Interpret(_ context.Context, req appinterfaces.LLMRequest) (appinterfaces.LLMResponse, error) {
	if req.JSONSchema == nil {
		return appinterfaces.LLMResponse{}, nil
	}
	if len(s.queue) == 0 {
		return appinterfaces.LLMResponse{}, nil
	}
	next := s.queue[0]
	s.queue = s.queue[1:]
	return next, nil
}

type fakeOnboardingOutboxPublisher struct{}

func (fakeOnboardingOutboxPublisher) Publish(_ context.Context, _ outbox.Event) error {
	return nil
}

type fakeOnboardingExpenseLogger struct{}

func (fakeOnboardingExpenseLogger) Execute(_ context.Context, _ tools.ExpenseRecorderInput) (tools.ExpenseRecorderResult, error) {
	return tools.ExpenseRecorderResult{Persisted: true, AmountCents: 3500, CategoryPath: "Custo Fixo"}, nil
}

func newOnboardingTurnPipeline(t *testing.T, db *sqlx.DB, interp appusecases.IntentInterpreter) *appusecases.RunOnboardingTurn {
	t.Helper()
	o11y := noop.NewProvider()
	publisher := fakeOnboardingOutboxPublisher{}
	idGen := id.NewUUIDGenerator()
	factory := repositories.NewRepositoryFactory(o11y)

	getContext := onbusecases.NewGetOnboardingContext(factory.OnboardingSessionRepository(db), o11y)
	saveObjective := onbusecases.NewSaveOnboardingObjective(uow.NewUnitOfWork(db), factory, o11y)
	saveIncome := onbusecases.NewSaveOnboardingIncome(uow.NewUnitOfWork(db), factory, o11y)
	saveCard := onbusecases.NewSaveOnboardingCard(uow.NewUnitOfWork(db), factory, publisher, idGen, o11y, nil)
	saveSplits := onbusecases.NewSaveOnboardingBudgetSplits(uow.NewUnitOfWork(db), factory, publisher, idGen, o11y)
	markFirstTx := onbusecases.NewMarkFirstTransactionRecorded(uow.NewUnitOfWork(db), factory, o11y)
	complete := onbusecases.NewCompleteOnboardingSession(uow.NewUnitOfWork(db), factory, publisher, idGen, o11y)
	setPhase := onbusecases.NewSetOnboardingPhase(uow.NewUnitOfWork(db), factory, o11y)

	reader := agentonboarding.NewOnboardingStateReader(getContext)
	require.NotNil(t, reader)
	phaseSetter := agentonboarding.NewOnboardingPhaseSetter(setPhase)
	require.NotNil(t, phaseSetter)
	dispatcher := agentonboarding.NewOnboardingToolDispatcher(saveObjective, saveIncome, saveCard, saveSplits, markFirstTx, complete, fakeOnboardingExpenseLogger{})

	agentSessionRepo := agentrepo.NewRepositoryFactory(o11y).AgentSessionRepository(db)
	v2session := agentbinding.NewOnboardingSessionGateway(agentSessionRepo)
	turn, err := appusecases.NewRunOnboardingTurn(interp, reader, dispatcher, phaseSetter, 512, o11y, nil, fakeSplitSuggester{}, v2session)
	require.NoError(t, err)
	return turn
}

func seedOnboardingSession(t *testing.T, db *sqlx.DB, userID uuid.UUID) {
	t.Helper()
	o11y := noop.NewProvider()
	repo := repositories.NewRepositoryFactory(o11y).OnboardingSessionRepository(db)
	session, err := onbentities.NewOnboardingSession(userID, onbentities.OnboardingChannelWhatsApp, time.Now().UTC())
	require.NoError(t, err)
	require.NoError(t, repo.Upsert(context.Background(), session))
}

func structuredResponse(action string, fields map[string]any) appinterfaces.LLMResponse {
	payload := map[string]any{"action": action, "reply": ""}
	for k, v := range fields {
		payload[k] = v
	}
	raw, _ := json.Marshal(payload)
	return appinterfaces.LLMResponse{RawJSON: raw}
}

func queryOnboardingString(t *testing.T, db *sqlx.DB, userID uuid.UUID, expr string) (string, bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var value *string
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT `+expr+` FROM mecontrola.onboarding_sessions WHERE user_id = $1`, userID,
	).Scan(&value))
	if value == nil {
		return "", false
	}
	return *value, true
}

func queryOnboardingInt(t *testing.T, db *sqlx.DB, userID uuid.UUID, expr string) (int64, bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var value *int64
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT `+expr+` FROM mecontrola.onboarding_sessions WHERE user_id = $1`, userID,
	).Scan(&value))
	if value == nil {
		return 0, false
	}
	return *value, true
}

func requireOnboardingPhase(t *testing.T, db *sqlx.DB, userID uuid.UUID, expected string) {
	t.Helper()
	got, _ := queryOnboardingString(t, db, userID, "payload->>'phase'")
	require.Equal(t, expected, got, "fase persistida divergente")
}

func conversationalTurn(t *testing.T, turn *appusecases.RunOnboardingTurn, userID uuid.UUID, text string) appusecases.RunOnboardingTurnResult {
	t.Helper()
	out, err := turn.Execute(context.Background(), appusecases.RunOnboardingTurnInput{UserID: userID, Channel: "whatsapp", Text: text})
	require.NoError(t, err)
	require.True(t, out.Handled)
	return out
}

func TestOnboardingConversational_Journey_E2E(t *testing.T) {
	db, _ := postgres.NewTestDatabase(t)

	userID := SeedActiveUserWA(t, db, "+5511955554444")
	seedOnboardingSession(t, db, userID)

	interp := &scriptedInterpreter{queue: []appinterfaces.LLMResponse{
		structuredResponse("save_onboarding_objective", map[string]any{"objective": "fazer uma viagem", "objective_profile": "specific_goal"}),
		structuredResponse("save_onboarding_income", map[string]any{"income_cents": 500000}),
		structuredResponse("record_transaction", map[string]any{"direction": "outcome", "amount_cents": 3500, "merchant": "mercado", "category_hint": "mercado"}),
	}}
	turn := newOnboardingTurnPipeline(t, db, interp)

	welcome := conversationalTurn(t, turn, userID, "oi")
	require.Contains(t, welcome.Reply, "Eu sou o *MeControla*")
	require.Contains(t, welcome.Reply, "Custo Fixo")
	requireOnboardingPhase(t, db, userID, "objective")

	objective := conversationalTurn(t, turn, userID, "quero fazer uma viagem")
	require.Contains(t, objective.Reply, "Etapa 2/4")
	requireOnboardingPhase(t, db, userID, "budget")
	gotObjective, _ := queryOnboardingString(t, db, userID, "payload->>'objective'")
	require.Equal(t, "fazer uma viagem", gotObjective)

	budget := conversationalTurn(t, turn, userID, "ganho 5000")
	require.Contains(t, budget.Reply, "R$ 1.750,00")
	require.Contains(t, budget.Reply, "Etapa 3/4")
	requireOnboardingPhase(t, db, userID, "cards")
	gotIncome, _ := queryOnboardingInt(t, db, userID, "(payload->>'income_cents')::bigint")
	require.Equal(t, int64(500000), gotIncome)

	cards := conversationalTurn(t, turn, userID, "não uso cartão")
	require.Contains(t, cards.Reply, "Etapa 4/4")
	requireOnboardingPhase(t, db, userID, "financial_plan")

	financialPlan := conversationalTurn(t, turn, userID, "sim")
	require.Contains(t, financialPlan.Reply, "primeiro lançamento")
	requireOnboardingPhase(t, db, userID, "first_tx")
	splitCount, _ := queryOnboardingInt(t, db, userID, "jsonb_array_length(payload->'custom_split')")
	require.Equal(t, int64(5), splitCount)

	firstTx := conversationalTurn(t, turn, userID, "gastei 35 no mercado")
	require.Contains(t, firstTx.Reply, "🏆 Boa! Registrei")
	require.Contains(t, firstTx.Reply, "Onboarding concluído")
	finalState, _ := queryOnboardingString(t, db, userID, "state")
	require.Equal(t, "active", finalState)
}
