package usecases

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
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
	snapshot OnboardingSnapshot
	err      error
}

func (f *fakeStateReader) Load(_ context.Context, _ uuid.UUID) (OnboardingSnapshot, error) {
	return f.snapshot, f.err
}

type fakeToolDispatcher struct {
	calls   int
	results map[string]OnboardingToolResult
	err     error
}

func (f *fakeToolDispatcher) Dispatch(_ context.Context, _ uuid.UUID, _ string, call interfaces.ToolCall) (OnboardingToolResult, error) {
	f.calls++
	if f.err != nil {
		return OnboardingToolResult{}, f.err
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

type RunOnboardingTurnSuite struct {
	suite.Suite
	ctx context.Context
}

func TestRunOnboardingTurnSuite(t *testing.T) {
	suite.Run(t, new(RunOnboardingTurnSuite))
}

func (s *RunOnboardingTurnSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *RunOnboardingTurnSuite) newTurn(interp IntentInterpreter, reader OnboardingStateReader, dispatcher OnboardingToolDispatcher, phases OnboardingPhaseSetter) *RunOnboardingTurn {
	uc, err := NewRunOnboardingTurn(interp, reader, dispatcher, phases, 512, fake.NewProvider())
	s.Require().NoError(err)
	return uc
}

func (s *RunOnboardingTurnSuite) TestNotInProgress() {
	interp := &fakeTurnInterpreter{}
	setter := &fakePhaseSetter{}
	uc := s.newTurn(interp, &fakeStateReader{snapshot: OnboardingSnapshot{InProgress: false}}, &fakeToolDispatcher{}, setter)
	out, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "oi"})
	s.Require().NoError(err)
	s.False(out.Handled)
	s.False(interp.called)
	s.Empty(setter.phases)
}

func (s *RunOnboardingTurnSuite) TestReaderError() {
	uc := s.newTurn(&fakeTurnInterpreter{}, &fakeStateReader{err: errors.New("boom")}, &fakeToolDispatcher{}, &fakePhaseSetter{})
	_, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "oi"})
	s.Require().Error(err)
}

func (s *RunOnboardingTurnSuite) TestNewSessionEmitsWelcome() {
	interp := &fakeTurnInterpreter{}
	setter := &fakePhaseSetter{}
	uc := s.newTurn(interp, &fakeStateReader{snapshot: OnboardingSnapshot{InProgress: true, Phase: ""}}, &fakeToolDispatcher{}, setter)
	out, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "oi"})
	s.Require().NoError(err)
	s.True(out.Handled)
	s.Contains(out.Reply, "Eu sou o *MeControla*")
	s.Equal(OnbPhaseWelcome, setter.last())
	s.False(interp.called)
}

func (s *RunOnboardingTurnSuite) TestWelcomeAffirmationAdvancesToMethodology() {
	setter := &fakePhaseSetter{}
	uc := s.newTurn(&fakeTurnInterpreter{}, &fakeStateReader{snapshot: OnboardingSnapshot{InProgress: true, Phase: OnbPhaseWelcome}}, &fakeToolDispatcher{}, setter)
	out, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "sim"})
	s.Require().NoError(err)
	s.True(out.Handled)
	s.Contains(out.Reply, "Custo Fixo")
	s.Equal(OnbPhaseMethodology1, setter.last())
}

func (s *RunOnboardingTurnSuite) TestMethodologyAdvancesOnNonQuestionReply() {
	for _, reply := range []string{"Faz", "faz sentido", "ok", "show", "👍", "entendi", "bora"} {
		setter := &fakePhaseSetter{}
		uc := s.newTurn(&fakeTurnInterpreter{}, &fakeStateReader{snapshot: OnboardingSnapshot{InProgress: true, Phase: OnbPhaseMethodology1}}, &fakeToolDispatcher{}, setter)
		out, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: reply})
		s.Require().NoError(err)
		s.True(out.Handled)
		s.Contains(out.Reply, "Conhecimento", "reply %q deveria avançar", reply)
		s.Equal(OnbPhaseMethodology2, setter.last(), "reply %q deveria avançar", reply)
	}
}

func (s *RunOnboardingTurnSuite) TestMethodologyNonAffirmationReasks() {
	setter := &fakePhaseSetter{}
	uc := s.newTurn(&fakeTurnInterpreter{}, &fakeStateReader{snapshot: OnboardingSnapshot{InProgress: true, Phase: OnbPhaseMethodology1}}, &fakeToolDispatcher{}, setter)
	out, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "espera, o que é isso?"})
	s.Require().NoError(err)
	s.True(out.Handled)
	s.Contains(out.Reply, "Custo Fixo")
	s.Empty(setter.phases)
}

func (s *RunOnboardingTurnSuite) TestObjectiveToolAdvancesToIncome() {
	interp := &fakeTurnInterpreter{resp: interfaces.LLMResponse{ToolCalls: []interfaces.ToolCall{{FunctionName: ToolSaveOnboardingObjective}}}}
	dispatcher := &fakeToolDispatcher{results: map[string]OnboardingToolResult{
		ToolSaveOnboardingObjective: {Reply: "🎯 Anotado!", Advance: true},
	}}
	setter := &fakePhaseSetter{}
	uc := s.newTurn(interp, &fakeStateReader{snapshot: OnboardingSnapshot{InProgress: true, Phase: OnbPhaseObjective}}, dispatcher, setter)
	out, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "quero fazer uma viagem"})
	s.Require().NoError(err)
	s.True(out.Handled)
	s.Contains(out.Reply, "🎯 Anotado!")
	s.Contains(out.Reply, "orçamento mensal")
	s.Equal(OnbPhaseIncome, setter.last())
}

func (s *RunOnboardingTurnSuite) TestObjectiveQuestionStaysNoAdvance() {
	interp := &fakeTurnInterpreter{resp: interfaces.LLMResponse{RawJSON: []byte("Pra te ajudar melhor com seu **objetivo**, me conta? 😊")}}
	setter := &fakePhaseSetter{}
	uc := s.newTurn(interp, &fakeStateReader{snapshot: OnboardingSnapshot{InProgress: true, Phase: OnbPhaseObjective}}, &fakeToolDispatcher{results: map[string]OnboardingToolResult{}}, setter)
	out, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "por que você precisa disso?"})
	s.Require().NoError(err)
	s.True(out.Handled)
	s.True(strings.Contains(out.Reply, "objetivo"))
	s.NotContains(out.Reply, "**")
	s.Contains(out.Reply, "*objetivo*")
	s.Empty(setter.phases)
}

func (s *RunOnboardingTurnSuite) TestCardsNegationAdvancesToSplits() {
	setter := &fakePhaseSetter{}
	uc := s.newTurn(&fakeTurnInterpreter{}, &fakeStateReader{snapshot: OnboardingSnapshot{InProgress: true, Phase: OnbPhaseCards}}, &fakeToolDispatcher{}, setter)
	out, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "não uso cartão"})
	s.Require().NoError(err)
	s.True(out.Handled)
	s.Contains(out.Reply, "distribuir seu orçamento")
	s.Equal(OnbPhaseSplits, setter.last())
}

func (s *RunOnboardingTurnSuite) TestSplitsDefaultAppliedWithoutLLM() {
	interp := &fakeTurnInterpreter{}
	dispatcher := &fakeToolDispatcher{results: map[string]OnboardingToolResult{
		ToolSaveOnboardingBudgetSplits: {Reply: "✅ Distribuição salva!", Advance: true},
	}}
	setter := &fakePhaseSetter{}
	uc := s.newTurn(interp, &fakeStateReader{snapshot: OnboardingSnapshot{InProgress: true, Phase: OnbPhaseSplits, IncomeCents: 500000}}, dispatcher, setter)
	out, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "sim, pode usar essa"})
	s.Require().NoError(err)
	s.True(out.Handled)
	s.False(interp.called)
	s.Equal(1, dispatcher.calls)
	s.Contains(out.Reply, "Distribuição salva")
	s.Equal(OnbPhaseSummary, setter.last())
}

func (s *RunOnboardingTurnSuite) TestSplitsCustomUsesLLM() {
	interp := &fakeTurnInterpreter{resp: interfaces.LLMResponse{ToolCalls: []interfaces.ToolCall{{FunctionName: ToolSaveOnboardingBudgetSplits}}}}
	dispatcher := &fakeToolDispatcher{results: map[string]OnboardingToolResult{
		ToolSaveOnboardingBudgetSplits: {Reply: "✅ Distribuição salva!", Advance: true},
	}}
	setter := &fakePhaseSetter{}
	uc := s.newTurn(interp, &fakeStateReader{snapshot: OnboardingSnapshot{InProgress: true, Phase: OnbPhaseSplits, IncomeCents: 500000}}, dispatcher, setter)
	out, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "custo fixo 5000, conhecimento 1000, prazeres 1500, metas 3000, liberdade 2000"})
	s.Require().NoError(err)
	s.True(out.Handled)
	s.True(interp.called)
	s.Equal(OnbPhaseSummary, setter.last())
}

func (s *RunOnboardingTurnSuite) TestSummaryAffirmationAdvancesToFirstTx() {
	setter := &fakePhaseSetter{}
	uc := s.newTurn(&fakeTurnInterpreter{}, &fakeStateReader{snapshot: OnboardingSnapshot{InProgress: true, Phase: OnbPhaseSummary}}, &fakeToolDispatcher{}, setter)
	out, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "tá perfeito"})
	s.Require().NoError(err)
	s.True(out.Handled)
	s.Contains(out.Reply, "primeiro lançamento")
	s.Equal(OnbPhaseFirstTx, setter.last())
}

func (s *RunOnboardingTurnSuite) TestFirstTxRecordsAndCompletes() {
	interp := &fakeTurnInterpreter{resp: interfaces.LLMResponse{ToolCalls: []interfaces.ToolCall{{FunctionName: "record_transaction"}}}}
	dispatcher := &fakeToolDispatcher{results: map[string]OnboardingToolResult{
		"record_transaction": {Reply: "🏆 Boa!\n\n🎉 *Onboarding concluído!*", Advance: true, Terminal: true},
	}}
	setter := &fakePhaseSetter{}
	uc := s.newTurn(interp, &fakeStateReader{snapshot: OnboardingSnapshot{InProgress: true, Phase: OnbPhaseFirstTx}}, dispatcher, setter)
	out, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "gastei 35 no mercado"})
	s.Require().NoError(err)
	s.True(out.Handled)
	s.Contains(out.Reply, "Onboarding concluído")
	s.Equal(1, dispatcher.calls)
}

func (s *RunOnboardingTurnSuite) TestInterpretErrorAtDataPhase() {
	interp := &fakeTurnInterpreter{err: errors.New("provider down")}
	uc := s.newTurn(interp, &fakeStateReader{snapshot: OnboardingSnapshot{InProgress: true, Phase: OnbPhaseObjective}}, &fakeToolDispatcher{}, &fakePhaseSetter{})
	_, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "viagem"})
	s.Require().Error(err)
}
