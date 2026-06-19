package usecases_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
)

type fakeTurnInterpreter struct {
	resp   interfaces.LLMResponse
	err    error
	called bool
}

func (f *fakeTurnInterpreter) Interpret(_ context.Context, _ interfaces.LLMRequest) (interfaces.LLMResponse, error) {
	f.called = true
	return f.resp, f.err
}

type fakeStateReader struct {
	snapshot usecases.OnboardingSnapshot
	err      error
}

func (f *fakeStateReader) Load(_ context.Context, _ uuid.UUID) (usecases.OnboardingSnapshot, error) {
	return f.snapshot, f.err
}

type fakeToolDispatcher struct {
	calls   int
	results map[string]usecases.OnboardingToolResult
	err     error
}

func (f *fakeToolDispatcher) Dispatch(_ context.Context, _ uuid.UUID, _ string, call interfaces.ToolCall) (usecases.OnboardingToolResult, error) {
	f.calls++
	if f.err != nil {
		return usecases.OnboardingToolResult{}, f.err
	}
	return f.results[call.FunctionName], nil
}

type fakePhaseSetter struct {
	mu     sync.Mutex
	phases []string
}

func (f *fakePhaseSetter) SetPhase(_ context.Context, _ uuid.UUID, phase string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.phases = append(f.phases, phase)
	return nil
}

func (f *fakePhaseSetter) last() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.phases) == 0 {
		return ""
	}
	return f.phases[len(f.phases)-1]
}

func newTurn(t *testing.T, interp usecases.IntentInterpreter, reader usecases.OnboardingStateReader, dispatcher usecases.OnboardingToolDispatcher, phases usecases.OnboardingPhaseSetter) *usecases.RunOnboardingTurn {
	t.Helper()
	uc, err := usecases.NewRunOnboardingTurn(interp, reader, dispatcher, phases, 512, noop.NewProvider())
	require.NoError(t, err)
	return uc
}

func TestRunOnboardingTurn_NotInProgress(t *testing.T) {
	t.Parallel()
	interp := &fakeTurnInterpreter{}
	setter := &fakePhaseSetter{}
	uc := newTurn(t, interp, &fakeStateReader{snapshot: usecases.OnboardingSnapshot{InProgress: false}}, &fakeToolDispatcher{}, setter)
	out, err := uc.Execute(context.Background(), usecases.RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "oi"})
	require.NoError(t, err)
	require.False(t, out.Handled)
	require.False(t, interp.called)
	require.Empty(t, setter.phases)
}

func TestRunOnboardingTurn_ReaderError(t *testing.T) {
	t.Parallel()
	uc := newTurn(t, &fakeTurnInterpreter{}, &fakeStateReader{err: errors.New("boom")}, &fakeToolDispatcher{}, &fakePhaseSetter{})
	_, err := uc.Execute(context.Background(), usecases.RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "oi"})
	require.Error(t, err)
}

func TestRunOnboardingTurn_NewSessionEmitsWelcome(t *testing.T) {
	t.Parallel()
	interp := &fakeTurnInterpreter{}
	setter := &fakePhaseSetter{}
	uc := newTurn(t, interp, &fakeStateReader{snapshot: usecases.OnboardingSnapshot{InProgress: true, Phase: ""}}, &fakeToolDispatcher{}, setter)
	out, err := uc.Execute(context.Background(), usecases.RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "oi"})
	require.NoError(t, err)
	require.True(t, out.Handled)
	require.Contains(t, out.Reply, "Eu sou o *MeControla*")
	require.Equal(t, usecases.OnbPhaseWelcome, setter.last())
	require.False(t, interp.called)
}

func TestRunOnboardingTurn_WelcomeAffirmationAdvancesToMethodology(t *testing.T) {
	t.Parallel()
	setter := &fakePhaseSetter{}
	uc := newTurn(t, &fakeTurnInterpreter{}, &fakeStateReader{snapshot: usecases.OnboardingSnapshot{InProgress: true, Phase: usecases.OnbPhaseWelcome}}, &fakeToolDispatcher{}, setter)
	out, err := uc.Execute(context.Background(), usecases.RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "sim"})
	require.NoError(t, err)
	require.True(t, out.Handled)
	require.Contains(t, out.Reply, "Custo Fixo")
	require.Equal(t, usecases.OnbPhaseMethodology1, setter.last())
}

func TestRunOnboardingTurn_MethodologyAdvancesOnNonQuestionReply(t *testing.T) {
	t.Parallel()
	for _, reply := range []string{"Faz", "faz sentido", "ok", "show", "👍", "entendi", "bora"} {
		setter := &fakePhaseSetter{}
		uc := newTurn(t, &fakeTurnInterpreter{}, &fakeStateReader{snapshot: usecases.OnboardingSnapshot{InProgress: true, Phase: usecases.OnbPhaseMethodology1}}, &fakeToolDispatcher{}, setter)
		out, err := uc.Execute(context.Background(), usecases.RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: reply})
		require.NoError(t, err)
		require.True(t, out.Handled)
		require.Contains(t, out.Reply, "Conhecimento", "reply %q deveria avançar", reply)
		require.Equal(t, usecases.OnbPhaseMethodology2, setter.last(), "reply %q deveria avançar", reply)
	}
}

func TestRunOnboardingTurn_MethodologyNonAffirmationReasks(t *testing.T) {
	t.Parallel()
	setter := &fakePhaseSetter{}
	uc := newTurn(t, &fakeTurnInterpreter{}, &fakeStateReader{snapshot: usecases.OnboardingSnapshot{InProgress: true, Phase: usecases.OnbPhaseMethodology1}}, &fakeToolDispatcher{}, setter)
	out, err := uc.Execute(context.Background(), usecases.RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "espera, o que é isso?"})
	require.NoError(t, err)
	require.True(t, out.Handled)
	require.Contains(t, out.Reply, "Custo Fixo")
	require.Empty(t, setter.phases)
}

func TestRunOnboardingTurn_ObjectiveToolAdvancesToIncome(t *testing.T) {
	t.Parallel()
	interp := &fakeTurnInterpreter{resp: interfaces.LLMResponse{ToolCalls: []interfaces.ToolCall{{FunctionName: usecases.ToolSaveOnboardingObjective}}}}
	dispatcher := &fakeToolDispatcher{results: map[string]usecases.OnboardingToolResult{
		usecases.ToolSaveOnboardingObjective: {Reply: "🎯 Anotado!", Advance: true},
	}}
	setter := &fakePhaseSetter{}
	uc := newTurn(t, interp, &fakeStateReader{snapshot: usecases.OnboardingSnapshot{InProgress: true, Phase: usecases.OnbPhaseObjective}}, dispatcher, setter)
	out, err := uc.Execute(context.Background(), usecases.RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "quero fazer uma viagem"})
	require.NoError(t, err)
	require.True(t, out.Handled)
	require.Contains(t, out.Reply, "🎯 Anotado!")
	require.Contains(t, out.Reply, "orçamento mensal")
	require.Equal(t, usecases.OnbPhaseIncome, setter.last())
}

func TestRunOnboardingTurn_ObjectiveQuestionStaysNoAdvance(t *testing.T) {
	t.Parallel()
	interp := &fakeTurnInterpreter{resp: interfaces.LLMResponse{RawJSON: []byte("Pra te ajudar melhor com seu **objetivo**, me conta? 😊")}}
	setter := &fakePhaseSetter{}
	uc := newTurn(t, interp, &fakeStateReader{snapshot: usecases.OnboardingSnapshot{InProgress: true, Phase: usecases.OnbPhaseObjective}}, &fakeToolDispatcher{results: map[string]usecases.OnboardingToolResult{}}, setter)
	out, err := uc.Execute(context.Background(), usecases.RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "por que você precisa disso?"})
	require.NoError(t, err)
	require.True(t, out.Handled)
	require.True(t, strings.Contains(out.Reply, "objetivo"))
	require.NotContains(t, out.Reply, "**")
	require.Contains(t, out.Reply, "*objetivo*")
	require.Empty(t, setter.phases)
}

func TestRunOnboardingTurn_CardsNegationAdvancesToSplits(t *testing.T) {
	t.Parallel()
	setter := &fakePhaseSetter{}
	uc := newTurn(t, &fakeTurnInterpreter{}, &fakeStateReader{snapshot: usecases.OnboardingSnapshot{InProgress: true, Phase: usecases.OnbPhaseCards}}, &fakeToolDispatcher{}, setter)
	out, err := uc.Execute(context.Background(), usecases.RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "não uso cartão"})
	require.NoError(t, err)
	require.True(t, out.Handled)
	require.Contains(t, out.Reply, "distribuir seu orçamento")
	require.Equal(t, usecases.OnbPhaseSplits, setter.last())
}

func TestRunOnboardingTurn_SplitsDefaultAppliedWithoutLLM(t *testing.T) {
	t.Parallel()
	interp := &fakeTurnInterpreter{}
	dispatcher := &fakeToolDispatcher{results: map[string]usecases.OnboardingToolResult{
		usecases.ToolSaveOnboardingBudgetSplits: {Reply: "✅ Distribuição salva!", Advance: true},
	}}
	setter := &fakePhaseSetter{}
	uc := newTurn(t, interp, &fakeStateReader{snapshot: usecases.OnboardingSnapshot{InProgress: true, Phase: usecases.OnbPhaseSplits, IncomeCents: 500000}}, dispatcher, setter)
	out, err := uc.Execute(context.Background(), usecases.RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "sim, pode usar essa"})
	require.NoError(t, err)
	require.True(t, out.Handled)
	require.False(t, interp.called)
	require.Equal(t, 1, dispatcher.calls)
	require.Contains(t, out.Reply, "Distribuição salva")
	require.Equal(t, usecases.OnbPhaseSummary, setter.last())
}

func TestRunOnboardingTurn_SplitsCustomUsesLLM(t *testing.T) {
	t.Parallel()
	interp := &fakeTurnInterpreter{resp: interfaces.LLMResponse{ToolCalls: []interfaces.ToolCall{{FunctionName: usecases.ToolSaveOnboardingBudgetSplits}}}}
	dispatcher := &fakeToolDispatcher{results: map[string]usecases.OnboardingToolResult{
		usecases.ToolSaveOnboardingBudgetSplits: {Reply: "✅ Distribuição salva!", Advance: true},
	}}
	setter := &fakePhaseSetter{}
	uc := newTurn(t, interp, &fakeStateReader{snapshot: usecases.OnboardingSnapshot{InProgress: true, Phase: usecases.OnbPhaseSplits, IncomeCents: 500000}}, dispatcher, setter)
	out, err := uc.Execute(context.Background(), usecases.RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "custo fixo 5000, conhecimento 1000, prazeres 1500, metas 3000, liberdade 2000"})
	require.NoError(t, err)
	require.True(t, out.Handled)
	require.True(t, interp.called)
	require.Equal(t, usecases.OnbPhaseSummary, setter.last())
}

func TestRunOnboardingTurn_SummaryAffirmationAdvancesToFirstTx(t *testing.T) {
	t.Parallel()
	setter := &fakePhaseSetter{}
	uc := newTurn(t, &fakeTurnInterpreter{}, &fakeStateReader{snapshot: usecases.OnboardingSnapshot{InProgress: true, Phase: usecases.OnbPhaseSummary}}, &fakeToolDispatcher{}, setter)
	out, err := uc.Execute(context.Background(), usecases.RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "tá perfeito"})
	require.NoError(t, err)
	require.True(t, out.Handled)
	require.Contains(t, out.Reply, "primeiro lançamento")
	require.Equal(t, usecases.OnbPhaseFirstTx, setter.last())
}

func TestRunOnboardingTurn_FirstTxRecordsAndCompletes(t *testing.T) {
	t.Parallel()
	interp := &fakeTurnInterpreter{resp: interfaces.LLMResponse{ToolCalls: []interfaces.ToolCall{{FunctionName: "record_transaction"}}}}
	dispatcher := &fakeToolDispatcher{results: map[string]usecases.OnboardingToolResult{
		"record_transaction": {Reply: "🏆 Boa!\n\n🎉 *Onboarding concluído!*", Advance: true, Terminal: true},
	}}
	setter := &fakePhaseSetter{}
	uc := newTurn(t, interp, &fakeStateReader{snapshot: usecases.OnboardingSnapshot{InProgress: true, Phase: usecases.OnbPhaseFirstTx}}, dispatcher, setter)
	out, err := uc.Execute(context.Background(), usecases.RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "gastei 35 no mercado"})
	require.NoError(t, err)
	require.True(t, out.Handled)
	require.Contains(t, out.Reply, "Onboarding concluído")
	require.Equal(t, 1, dispatcher.calls)
}

func TestRunOnboardingTurn_InterpretErrorAtDataPhase(t *testing.T) {
	t.Parallel()
	interp := &fakeTurnInterpreter{err: errors.New("provider down")}
	uc := newTurn(t, interp, &fakeStateReader{snapshot: usecases.OnboardingSnapshot{InProgress: true, Phase: usecases.OnbPhaseObjective}}, &fakeToolDispatcher{}, &fakePhaseSetter{})
	_, err := uc.Execute(context.Background(), usecases.RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "viagem"})
	require.Error(t, err)
}
