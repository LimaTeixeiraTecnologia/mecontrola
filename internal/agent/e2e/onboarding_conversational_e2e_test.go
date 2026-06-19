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

func (fakeOnboardingExpenseLogger) Execute(_ context.Context, _ appservices.ExpenseLoggerInput) (appservices.ExpenseLoggerResult, error) {
	return appservices.ExpenseLoggerResult{Persisted: true, AmountCents: 3500, CategoryPath: "Custo Fixo"}, nil
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

	reader := agentonboarding.NewOnboardingStateReader(getContext)
	require.NotNil(t, reader)
	dispatcher := agentonboarding.NewOnboardingToolDispatcher(saveObjective, saveIncome, saveCard, saveSplits, markFirstTx, complete, fakeOnboardingExpenseLogger{})

	turn, err := appusecases.NewRunOnboardingTurn(interp, reader, dispatcher, 512, o11y)
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

func TestOnboardingConversational_Journey_E2E(t *testing.T) {
	db, _ := postgres.NewTestDatabase(t)
	ctx := context.Background()

	userID := SeedActiveUserWA(t, db, "+5511955554444")
	seedOnboardingSession(t, db, userID, onbvalueobjects.OnboardingStateAwaitingIncome)

	interp := &scriptedInterpreter{queue: []appinterfaces.LLMResponse{
		toolCallResponse("save_onboarding_objective", map[string]any{"objective": "fazer uma viagem"}),
		toolCallResponse("save_onboarding_income", map[string]any{"income_cents": 500000}),
		toolCallResponse("save_onboarding_card", map[string]any{"nickname": "nubank", "due_day": 17}),
		toolCallResponse("save_onboarding_budget_splits", map[string]any{
			"allocations": []any{
				map[string]any{"root_slug": "expense.custo_fixo", "amount_cents": 200000},
				map[string]any{"root_slug": "expense.conhecimento", "amount_cents": 50000},
				map[string]any{"root_slug": "expense.prazeres", "amount_cents": 75000},
				map[string]any{"root_slug": "expense.metas", "amount_cents": 100000},
				map[string]any{"root_slug": "expense.liberdade_financeira", "amount_cents": 75000},
			},
		}),
		{ToolCalls: []appinterfaces.ToolCall{
			{ID: "c1", FunctionName: "record_transaction", ArgumentsJSON: map[string]any{"direction": "outcome", "amount_cents": 3500, "merchant": "mercado"}},
		}},
	}}
	turn := newOnboardingTurnPipeline(t, db, interp)

	objective, err := turn.Execute(ctx, appusecases.RunOnboardingTurnInput{UserID: userID, Channel: "whatsapp", Text: "quero fazer uma viagem"})
	require.NoError(t, err)
	require.True(t, objective.Handled)
	require.Contains(t, objective.Reply, "Anotado: seu foco é **fazer uma viagem**")
	gotObjective, _ := queryOnboardingString(t, db, userID, "payload->>'objective'")
	require.Equal(t, "fazer uma viagem", gotObjective)

	income, err := turn.Execute(ctx, appusecases.RunOnboardingTurnInput{UserID: userID, Channel: "whatsapp", Text: "ganho 5000"})
	require.NoError(t, err)
	require.True(t, income.Handled)
	require.Equal(t, "✅ Orçamento de **R$ 5.000,00** registrado!", income.Reply)
	gotIncome, _ := queryOnboardingInt(t, db, userID, "(payload->>'income_cents')::bigint")
	require.Equal(t, int64(500000), gotIncome)

	cardOut, err := turn.Execute(ctx, appusecases.RunOnboardingTurnInput{UserID: userID, Channel: "whatsapp", Text: "tenho nubank dia 17"})
	require.NoError(t, err)
	require.True(t, cardOut.Handled)
	require.Contains(t, cardOut.Reply, "Cartão **nubank** salvo (vence dia 17")
	cardCount, _ := queryOnboardingInt(t, db, userID, "jsonb_array_length(payload->'cards')")
	require.Equal(t, int64(1), cardCount)

	splits, err := turn.Execute(ctx, appusecases.RunOnboardingTurnInput{UserID: userID, Channel: "whatsapp", Text: "distribui assim"})
	require.NoError(t, err)
	require.True(t, splits.Handled)
	require.Equal(t, "✅ Distribuição salva! 💰40% (R$2.000) · 🎓10% (R$500) · 🎉15% (R$750) · 🎯20% (R$1.000) · 🏦15% (R$750).", splits.Reply)
	stateAfterSplits, _ := queryOnboardingString(t, db, userID, "state")
	require.Equal(t, "awaiting_first_transaction", stateAfterSplits)
	splitCount, _ := queryOnboardingInt(t, db, userID, "jsonb_array_length(payload->'custom_split')")
	require.Equal(t, int64(5), splitCount)

	firstTx, err := turn.Execute(ctx, appusecases.RunOnboardingTurnInput{UserID: userID, Channel: "whatsapp", Text: "gastei 35 no mercado"})
	require.NoError(t, err)
	require.True(t, firstTx.Handled)
	require.Contains(t, firstTx.Reply, "🏆 Boa! Registrei")
	require.Contains(t, firstTx.Reply, "🎉 **Onboarding concluído!**")
	finalState, _ := queryOnboardingString(t, db, userID, "state")
	require.Equal(t, "active", finalState)
}

func TestOnboardingConversational_SplitsMismatch_E2E(t *testing.T) {
	db, _ := postgres.NewTestDatabase(t)
	ctx := context.Background()

	userID := SeedActiveUserWA(t, db, "+5511944443333")
	seedOnboardingSession(t, db, userID, onbvalueobjects.OnboardingStateAwaitingIncome)

	interp := &scriptedInterpreter{queue: []appinterfaces.LLMResponse{
		toolCallResponse("save_onboarding_income", map[string]any{"income_cents": 500000}),
		toolCallResponse("save_onboarding_budget_splits", map[string]any{
			"allocations": []any{
				map[string]any{"root_slug": "expense.custo_fixo", "amount_cents": 300000},
				map[string]any{"root_slug": "expense.conhecimento", "amount_cents": 50000},
				map[string]any{"root_slug": "expense.prazeres", "amount_cents": 75000},
				map[string]any{"root_slug": "expense.metas", "amount_cents": 100000},
				map[string]any{"root_slug": "expense.liberdade_financeira", "amount_cents": 75000},
			},
		}),
	}}
	turn := newOnboardingTurnPipeline(t, db, interp)

	income, err := turn.Execute(ctx, appusecases.RunOnboardingTurnInput{UserID: userID, Channel: "whatsapp", Text: "ganho 5000"})
	require.NoError(t, err)
	require.True(t, income.Handled)

	splits, err := turn.Execute(ctx, appusecases.RunOnboardingTurnInput{UserID: userID, Channel: "whatsapp", Text: "distribui assim"})
	require.NoError(t, err)
	require.True(t, splits.Handled)
	require.Contains(t, splits.Reply, "passou **R$ 1.000**")

	splitCount, present := queryOnboardingInt(t, db, userID, "jsonb_array_length(payload->'custom_split')")
	require.True(t, !present || splitCount == 0)
	state, _ := queryOnboardingString(t, db, userID, "state")
	require.NotEqual(t, "active", state)
}

func TestOnboardingConversational_CompleteBlockedWithoutFirstTx_E2E(t *testing.T) {
	db, _ := postgres.NewTestDatabase(t)
	ctx := context.Background()

	userID := SeedActiveUserWA(t, db, "+5511933332222")
	seedOnboardingSession(t, db, userID, onbvalueobjects.OnboardingStateAwaitingIncome)

	prep := &scriptedInterpreter{queue: []appinterfaces.LLMResponse{
		toolCallResponse("save_onboarding_objective", map[string]any{"objective": "fazer uma viagem"}),
		toolCallResponse("save_onboarding_income", map[string]any{"income_cents": 500000}),
		toolCallResponse("save_onboarding_budget_splits", map[string]any{
			"allocations": []any{
				map[string]any{"root_slug": "expense.custo_fixo", "amount_cents": 200000},
				map[string]any{"root_slug": "expense.conhecimento", "amount_cents": 50000},
				map[string]any{"root_slug": "expense.prazeres", "amount_cents": 75000},
				map[string]any{"root_slug": "expense.metas", "amount_cents": 100000},
				map[string]any{"root_slug": "expense.liberdade_financeira", "amount_cents": 75000},
			},
		}),
	}}
	prepTurn := newOnboardingTurnPipeline(t, db, prep)
	for _, text := range []string{"viagem", "ganho 5000", "distribui"} {
		out, err := prepTurn.Execute(ctx, appusecases.RunOnboardingTurnInput{UserID: userID, Channel: "whatsapp", Text: text})
		require.NoError(t, err)
		require.True(t, out.Handled)
	}
	stateBefore, _ := queryOnboardingString(t, db, userID, "state")
	require.Equal(t, "awaiting_first_transaction", stateBefore)

	completeInterp := &scriptedInterpreter{queue: []appinterfaces.LLMResponse{
		toolCallResponse("complete_onboarding_session", map[string]any{}),
	}}
	completeTurn := newOnboardingTurnPipeline(t, db, completeInterp)

	out, err := completeTurn.Execute(ctx, appusecases.RunOnboardingTurnInput{UserID: userID, Channel: "whatsapp", Text: "terminei"})
	require.NoError(t, err)
	require.True(t, out.Handled)
	require.Contains(t, out.Reply, "faça seu primeiro lançamento")

	state, _ := queryOnboardingString(t, db, userID, "state")
	require.NotEqual(t, "active", state)
}

func TestOnboardingConversational_QuestionFreeText_E2E(t *testing.T) {
	db, _ := postgres.NewTestDatabase(t)
	ctx := context.Background()

	userID := SeedActiveUserWA(t, db, "+5511922221111")
	seedOnboardingSession(t, db, userID, onbvalueobjects.OnboardingStateAwaitingIncome)

	const answer = "🙂 Boa pergunta! Orçamento é tudo que entra por mês. Me conta: qual seu orçamento mensal?"
	interp := &scriptedInterpreter{queue: []appinterfaces.LLMResponse{
		{RawJSON: []byte(answer)},
	}}
	turn := newOnboardingTurnPipeline(t, db, interp)

	out, err := turn.Execute(ctx, appusecases.RunOnboardingTurnInput{UserID: userID, Channel: "whatsapp", Text: "o que é orçamento?"})
	require.NoError(t, err)
	require.True(t, out.Handled)
	require.Equal(t, answer, out.Reply)

	income, present := queryOnboardingInt(t, db, userID, "(payload->>'income_cents')::bigint")
	require.True(t, !present || income == 0)
	state, _ := queryOnboardingString(t, db, userID, "state")
	require.Equal(t, "awaiting_income", state)
}
