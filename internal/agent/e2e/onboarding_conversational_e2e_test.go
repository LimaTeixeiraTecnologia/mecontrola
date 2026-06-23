//go:build e2e

package e2e_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	appusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	agentonboarding "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/onboarding"
	onbusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	onbentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	onbvalueobjects "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
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

func (s *scriptedInterpreter) Interpret(_ context.Context, _ appinterfaces.LLMRequest) (appinterfaces.LLMResponse, error) {
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

func (fakeOnboardingExpenseLogger) Execute(_ context.Context, _ appservices.ExpenseRecorderInput) (appservices.ExpenseRecorderResult, error) {
	return appservices.ExpenseRecorderResult{Persisted: true, AmountCents: 3500, CategoryPath: "Custo Fixo"}, nil
}

func newOnboardingTurnPipeline(t *testing.T, db *sqlx.DB, interp appusecases.IntentInterpreter) *appusecases.RunOnboardingTurn {
	t.Helper()
	o11y := noop.NewProvider()
	publisher := fakeOnboardingOutboxPublisher{}
	idGen := id.NewUUIDGenerator()
	factory := repositories.NewRepositoryFactory(o11y)

	getContext := onbusecases.NewGetOnboardingContext(factory.OnboardingSessionRepository(db), o11y)
	saveObjective := onbusecases.NewSaveOnboardingObjective(uow.NewUnitOfWork(db), factory, o11y)
	saveIncome := onbusecases.NewSaveOnboardingIncome(uow.NewUnitOfWork(db), factory, publisher, idGen, o11y)
	saveCard := onbusecases.NewSaveOnboardingCard(uow.NewUnitOfWork(db), factory, publisher, idGen, o11y)
	saveSplits := onbusecases.NewSaveOnboardingBudgetSplits(uow.NewUnitOfWork(db), factory, publisher, idGen, o11y)
	markFirstTx := onbusecases.NewMarkFirstTransactionRecorded(uow.NewUnitOfWork(db), factory, o11y)
	complete := onbusecases.NewCompleteOnboardingSession(uow.NewUnitOfWork(db), factory, publisher, idGen, o11y)
	setPhase := onbusecases.NewSetOnboardingPhase(uow.NewUnitOfWork(db), factory, o11y)

	reader := agentonboarding.NewOnboardingStateReader(getContext)
	require.NotNil(t, reader)
	phaseSetter := agentonboarding.NewOnboardingPhaseSetter(setPhase)
	require.NotNil(t, phaseSetter)
	dispatcher := agentonboarding.NewOnboardingToolDispatcher(saveObjective, saveIncome, saveCard, saveSplits, markFirstTx, complete, fakeOnboardingExpenseLogger{})

	turn, err := appusecases.NewRunOnboardingTurn(interp, reader, dispatcher, phaseSetter, 512, o11y, nil)
	require.NoError(t, err)
	return turn
}

func seedOnboardingSession(t *testing.T, db *sqlx.DB, userID uuid.UUID, state onbvalueobjects.OnboardingState) {
	t.Helper()
	o11y := noop.NewProvider()
	repo := repositories.NewRepositoryFactory(o11y).OnboardingSessionRepository(db)
	session, err := onbentities.NewOnboardingSession(userID, onbentities.OnboardingChannelWhatsApp, state, time.Now().UTC())
	require.NoError(t, err)
	require.NoError(t, repo.Upsert(context.Background(), session))
}

func toolCallResponse(name string, args map[string]any) appinterfaces.LLMResponse {
	return appinterfaces.LLMResponse{
		ToolCalls: []appinterfaces.ToolCall{{ID: "c1", FunctionName: name, ArgumentsJSON: args}},
	}
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
	seedOnboardingSession(t, db, userID, onbvalueobjects.OnboardingStateAwaitingIncome)

	interp := &scriptedInterpreter{queue: []appinterfaces.LLMResponse{
		toolCallResponse("save_onboarding_objective", map[string]any{"objective": "fazer uma viagem"}),
		toolCallResponse("save_onboarding_income", map[string]any{"income_cents": 500000}),
		toolCallResponse("save_onboarding_card", map[string]any{"nickname": "nubank", "due_day": 17}),
		{ToolCalls: []appinterfaces.ToolCall{
			{ID: "c1", FunctionName: "record_transaction", ArgumentsJSON: map[string]any{"direction": "outcome", "amount_cents": 3500, "merchant": "mercado", "category_hint": "mercado"}},
		}},
	}}
	turn := newOnboardingTurnPipeline(t, db, interp)

	welcome := conversationalTurn(t, turn, userID, "oi")
	require.Contains(t, welcome.Reply, "Eu sou o *MeControla*")
	requireOnboardingPhase(t, db, userID, "welcome")

	m1 := conversationalTurn(t, turn, userID, "sim")
	require.Contains(t, m1.Reply, "Custo Fixo")
	requireOnboardingPhase(t, db, userID, "methodology_1")

	m2 := conversationalTurn(t, turn, userID, "sim")
	require.Contains(t, m2.Reply, "Conhecimento")
	requireOnboardingPhase(t, db, userID, "methodology_2")

	m3 := conversationalTurn(t, turn, userID, "sim")
	require.Contains(t, m3.Reply, "Prazeres")
	requireOnboardingPhase(t, db, userID, "methodology_3")

	m4 := conversationalTurn(t, turn, userID, "sim")
	require.Contains(t, m4.Reply, "Metas")
	requireOnboardingPhase(t, db, userID, "methodology_4")

	m5 := conversationalTurn(t, turn, userID, "sim")
	require.Contains(t, m5.Reply, "Liberdade Financeira")
	requireOnboardingPhase(t, db, userID, "methodology_5")

	objQuestion := conversationalTurn(t, turn, userID, "sim")
	require.Contains(t, objQuestion.Reply, "objetivo principal")
	requireOnboardingPhase(t, db, userID, "objective")

	objective := conversationalTurn(t, turn, userID, "quero fazer uma viagem")
	require.Contains(t, objective.Reply, "🎯 Anotado: seu foco é *fazer uma viagem*.")
	require.Contains(t, objective.Reply, "orçamento mensal")
	requireOnboardingPhase(t, db, userID, "income")
	gotObjective, _ := queryOnboardingString(t, db, userID, "payload->>'objective'")
	require.Equal(t, "fazer uma viagem", gotObjective)

	income := conversationalTurn(t, turn, userID, "ganho 5000")
	require.Contains(t, income.Reply, "✅ Orçamento de *R$ 5.000,00* registrado!")
	require.Contains(t, income.Reply, "cartão de crédito")
	requireOnboardingPhase(t, db, userID, "cards")
	gotIncome, _ := queryOnboardingInt(t, db, userID, "(payload->>'income_cents')::bigint")
	require.Equal(t, int64(500000), gotIncome)

	cardOut := conversationalTurn(t, turn, userID, "uso o nubank, vence dia 17")
	require.Contains(t, cardOut.Reply, "💳 Cartão *nubank* salvo (vence dia 17")
	requireOnboardingPhase(t, db, userID, "cards")
	cardCount, _ := queryOnboardingInt(t, db, userID, "jsonb_array_length(payload->'cards')")
	require.Equal(t, int64(1), cardCount)

	splitsQuestion := conversationalTurn(t, turn, userID, "não, só esse")
	require.Contains(t, splitsQuestion.Reply, "distribuir seu orçamento")
	requireOnboardingPhase(t, db, userID, "splits")

	splits := conversationalTurn(t, turn, userID, "distribui assim")
	require.Contains(t, splits.Reply, "✅ *Distribuição salva!*")
	require.Contains(t, splits.Reply, "💰 Custo Fixo: R$ 2.000,00 (40%)")
	require.Contains(t, splits.Reply, "🏦 Liberdade Financeira: R$ 750,00 (15%)")
	require.Contains(t, splits.Reply, "Seu plano:")
	require.Contains(t, splits.Reply, "Tá tudo certo?")
	requireOnboardingPhase(t, db, userID, "summary")
	splitCount, _ := queryOnboardingInt(t, db, userID, "jsonb_array_length(payload->'custom_split')")
	require.Equal(t, int64(5), splitCount)

	transition := conversationalTurn(t, turn, userID, "tá perfeito")
	require.Contains(t, transition.Reply, "primeiro lançamento")
	requireOnboardingPhase(t, db, userID, "first_tx")

	firstTx := conversationalTurn(t, turn, userID, "gastei 35 no mercado")
	require.Contains(t, firstTx.Reply, "🏆 Boa! Registrei")
	require.Contains(t, firstTx.Reply, "🎉 *Onboarding concluído!*")
	finalState, _ := queryOnboardingString(t, db, userID, "state")
	require.Equal(t, "active", finalState)
}

func TestOnboardingConversational_MethodologyStrictlyAdvances_E2E(t *testing.T) {
	db, _ := postgres.NewTestDatabase(t)

	userID := SeedActiveUserWA(t, db, "+5511944443333")
	seedOnboardingSession(t, db, userID, onbvalueobjects.OnboardingStateAwaitingIncome)

	turn := newOnboardingTurnPipeline(t, db, &scriptedInterpreter{})

	welcome := conversationalTurn(t, turn, userID, "oi")
	require.Contains(t, welcome.Reply, "Eu sou o *MeControla*")
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
		out := conversationalTurn(t, turn, userID, "sim")
		require.Contains(t, out.Reply, step.snippet)
		requireOnboardingPhase(t, db, userID, step.phase)
	}
}

func TestOnboardingConversational_MethodologyNonAffirmationStays_E2E(t *testing.T) {
	db, _ := postgres.NewTestDatabase(t)

	userID := SeedActiveUserWA(t, db, "+5511933332222")
	seedOnboardingSession(t, db, userID, onbvalueobjects.OnboardingStateAwaitingIncome)

	turn := newOnboardingTurnPipeline(t, db, &scriptedInterpreter{})

	conversationalTurn(t, turn, userID, "oi")
	requireOnboardingPhase(t, db, userID, "welcome")

	advance := conversationalTurn(t, turn, userID, "sim")
	require.Contains(t, advance.Reply, "Custo Fixo")
	requireOnboardingPhase(t, db, userID, "methodology_1")

	stay := conversationalTurn(t, turn, userID, "o que é isso?")
	require.Contains(t, stay.Reply, "Custo Fixo")
	requireOnboardingPhase(t, db, userID, "methodology_1")
}
