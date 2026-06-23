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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/onboardingv2draft"
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
	snapshots []OnboardingSnapshot
	idx       int
	err       error
}

func (f *fakeStateReader) Load(_ context.Context, _ uuid.UUID) (OnboardingSnapshot, error) {
	if f.err != nil {
		return OnboardingSnapshot{}, f.err
	}
	if f.idx >= len(f.snapshots) {
		return f.snapshots[len(f.snapshots)-1], nil
	}
	snap := f.snapshots[f.idx]
	f.idx++
	return snap, nil
}

func newReader(snaps ...OnboardingSnapshot) *fakeStateReader {
	return &fakeStateReader{snapshots: snaps}
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

type fakeV2Session struct {
	draft   onboardingv2draft.Draft
	found   bool
	saveErr error
	saved   *onboardingv2draft.Draft
	cleared bool
}

func (f *fakeV2Session) Load(_ context.Context, _ uuid.UUID, _ string) (onboardingv2draft.Draft, bool, error) {
	return f.draft, f.found, nil
}

func (f *fakeV2Session) Save(_ context.Context, _ uuid.UUID, _ string, d onboardingv2draft.Draft) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.saved = &d
	return nil
}

func (f *fakeV2Session) Clear(_ context.Context, _ uuid.UUID, _ string) error {
	f.cleared = true
	return nil
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

func (s *RunOnboardingTurnSuite) newTurn(interp IntentInterpreter, reader OnboardingStateReader, dispatcher OnboardingToolDispatcher, phases OnboardingPhaseSetter, v2 OnboardingV2SessionGateway) *RunOnboardingTurn {
	if v2 == nil {
		v2 = &fakeV2Session{}
	}
	uc, err := NewRunOnboardingTurn(interp, reader, dispatcher, phases, 512, fake.NewProvider(), nil, v2)
	s.Require().NoError(err)
	return uc
}

func (s *RunOnboardingTurnSuite) TestNotInProgress() {
	interp := &fakeTurnInterpreter{}
	setter := &fakePhaseSetter{}
	uc := s.newTurn(interp, newReader(OnboardingSnapshot{InProgress: false}), &fakeToolDispatcher{}, setter, nil)
	out, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "oi"})
	s.Require().NoError(err)
	s.False(out.Handled)
	s.False(interp.called)
	s.Empty(setter.phases)
}

func (s *RunOnboardingTurnSuite) TestReaderError() {
	uc := s.newTurn(&fakeTurnInterpreter{}, &fakeStateReader{err: errors.New("boom")}, &fakeToolDispatcher{}, &fakePhaseSetter{}, nil)
	_, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "oi"})
	s.Require().Error(err)
}

func (s *RunOnboardingTurnSuite) TestWelcomeOnEmptyPhase() {
	interp := &fakeTurnInterpreter{}
	setter := &fakePhaseSetter{}
	uc := s.newTurn(interp, newReader(OnboardingSnapshot{InProgress: true, Phase: ""}), &fakeToolDispatcher{}, setter, nil)
	out, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "oi"})
	s.Require().NoError(err)
	s.True(out.Handled)
	s.Contains(out.Reply, "Eu sou o *MeControla*")
	s.Contains(out.Reply, "Custo Fixo")
	s.Contains(out.Reply, "Conhecimento")
	s.Contains(out.Reply, "Prazeres")
	s.Contains(out.Reply, "Metas")
	s.Contains(out.Reply, "Liberdade Financeira")
	s.Contains(out.Reply, "Etapa 1/4")
	s.Equal(OnbPhaseObjective, setter.last())
	s.False(interp.called)
}

func (s *RunOnboardingTurnSuite) TestObjectiveAdvancesToBudget() {
	interp := &fakeTurnInterpreter{resp: interfaces.LLMResponse{ToolCalls: []interfaces.ToolCall{{FunctionName: ToolSaveOnboardingObjective}}}}
	dispatcher := &fakeToolDispatcher{results: map[string]OnboardingToolResult{
		ToolSaveOnboardingObjective: {Reply: "🎯 Objetivo anotado!", Advance: true},
	}}
	setter := &fakePhaseSetter{}
	v2 := &fakeV2Session{}
	uc := s.newTurn(interp, newReader(OnboardingSnapshot{InProgress: true, Phase: OnbPhaseObjective}), dispatcher, setter, v2)
	out, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "quitar dívidas"})
	s.Require().NoError(err)
	s.True(out.Handled)
	s.Contains(out.Reply, "🎯 Objetivo anotado!")
	s.Contains(out.Reply, "Etapa 2/4")
	s.Equal(OnbPhaseBudget, setter.last())
	s.NotNil(v2.saved)
	s.Equal(onboardingv2draft.StepBudget, v2.saved.Step())
}

func (s *RunOnboardingTurnSuite) TestObjectiveStayOnNoToolCall() {
	interp := &fakeTurnInterpreter{resp: interfaces.LLMResponse{RawJSON: []byte("Pode me contar mais? 😊")}}
	setter := &fakePhaseSetter{}
	uc := s.newTurn(interp, newReader(OnboardingSnapshot{InProgress: true, Phase: OnbPhaseObjective}), &fakeToolDispatcher{results: map[string]OnboardingToolResult{}}, setter, nil)
	out, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "não sei bem"})
	s.Require().NoError(err)
	s.True(out.Handled)
	s.Empty(setter.phases)
}

func (s *RunOnboardingTurnSuite) TestBudgetAutoGeneratesSplits() {
	interp := &fakeTurnInterpreter{resp: interfaces.LLMResponse{ToolCalls: []interfaces.ToolCall{{FunctionName: ToolSaveOnboardingIncome}}}}
	dispatcher := &fakeToolDispatcher{results: map[string]OnboardingToolResult{
		ToolSaveOnboardingIncome: {Reply: "💰 Orçamento salvo!", Advance: true},
	}}
	setter := &fakePhaseSetter{}
	v2 := &fakeV2Session{}
	snapAfter := OnboardingSnapshot{InProgress: true, Phase: OnbPhaseBudget, IncomeCents: 500000}
	uc := s.newTurn(interp,
		newReader(OnboardingSnapshot{InProgress: true, Phase: OnbPhaseBudget}, snapAfter),
		dispatcher, setter, v2)
	out, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "ganho 5000"})
	s.Require().NoError(err)
	s.True(out.Handled)
	s.Contains(out.Reply, "💰 Orçamento salvo!")
	s.Contains(out.Reply, "distribuição")
	s.Contains(out.Reply, "Etapa 3/4")
	s.Equal(OnbPhaseCards, setter.last())
	s.NotNil(v2.saved)
	s.True(v2.saved.HasAutoSplits())
	s.Len(v2.saved.Splits(), 5)
	var total int64
	for _, sp := range v2.saved.Splits() {
		total += sp.AmountCents
	}
	s.Equal(int64(500000), total)
}

func (s *RunOnboardingTurnSuite) TestCardsNegationSkipsToFinancialPlan() {
	setter := &fakePhaseSetter{}
	autoSplits := buildAutoSplits(500000)
	draft := onboardingv2draft.New().WithAutoSplits(autoSplits)
	v2 := &fakeV2Session{draft: draft, found: true}
	uc := s.newTurn(&fakeTurnInterpreter{}, newReader(OnboardingSnapshot{InProgress: true, Phase: OnbPhaseCards}), &fakeToolDispatcher{}, setter, v2)
	out, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "não uso cartão"})
	s.Require().NoError(err)
	s.True(out.Handled)
	s.Equal(OnbPhaseFinancialPlan, setter.last())
	s.Contains(out.Reply, "Etapa 4/4")
	s.Contains(out.Reply, "Plano Financeiro")
}

func (s *RunOnboardingTurnSuite) TestCardsMultiCardSingleTurn() {
	interp := &fakeTurnInterpreter{resp: interfaces.LLMResponse{ToolCalls: []interfaces.ToolCall{
		{FunctionName: ToolSaveOnboardingCard},
		{FunctionName: ToolSaveOnboardingCard},
	}}}
	dispatcher := &fakeToolDispatcher{results: map[string]OnboardingToolResult{
		ToolSaveOnboardingCard: {Reply: "💳 Cartão salvo!", Advance: true},
	}}
	setter := &fakePhaseSetter{}
	autoSplits := buildAutoSplits(500000)
	draft := onboardingv2draft.New().WithAutoSplits(autoSplits)
	v2 := &fakeV2Session{draft: draft, found: true}
	uc := s.newTurn(interp, newReader(OnboardingSnapshot{InProgress: true, Phase: OnbPhaseCards}), dispatcher, setter, v2)
	out, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "Nubank 13\nInter 5"})
	s.Require().NoError(err)
	s.True(out.Handled)
	s.Equal(2, dispatcher.calls)
	s.Equal(OnbPhaseFinancialPlan, setter.last())
	s.Contains(out.Reply, "Etapa 4/4")
}

func (s *RunOnboardingTurnSuite) TestFinancialPlanConfirmUsesAutoSplits() {
	dispatcher := &fakeToolDispatcher{results: map[string]OnboardingToolResult{
		ToolSaveOnboardingBudgetSplits: {Reply: "✅ Plano salvo!", Advance: true},
	}}
	setter := &fakePhaseSetter{}
	autoSplits := buildAutoSplits(500000)
	draft := onboardingv2draft.New().WithAutoSplits(autoSplits)
	v2 := &fakeV2Session{draft: draft, found: true}
	uc := s.newTurn(&fakeTurnInterpreter{}, newReader(OnboardingSnapshot{InProgress: true, Phase: OnbPhaseFinancialPlan, IncomeCents: 500000}), dispatcher, setter, v2)
	out, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "sim"})
	s.Require().NoError(err)
	s.True(out.Handled)
	s.Equal(1, dispatcher.calls)
	s.Equal(OnbPhaseFirstTx, setter.last())
	s.Contains(out.Reply, "primeiro lançamento")
	s.True(v2.cleared)
}

func (s *RunOnboardingTurnSuite) TestFinancialPlanAdjustPreservesAutoFlag() {
	interp := &fakeTurnInterpreter{resp: interfaces.LLMResponse{ToolCalls: []interfaces.ToolCall{{FunctionName: ToolSaveOnboardingBudgetSplits}}}}
	dispatcher := &fakeToolDispatcher{results: map[string]OnboardingToolResult{
		ToolSaveOnboardingBudgetSplits: {Reply: "✅ Distribuição ajustada!", Advance: true},
	}}
	setter := &fakePhaseSetter{}
	autoSplits := buildAutoSplits(500000)
	draft := onboardingv2draft.New().WithAutoSplits(autoSplits)
	v2 := &fakeV2Session{draft: draft, found: true}
	uc := s.newTurn(interp, newReader(OnboardingSnapshot{InProgress: true, Phase: OnbPhaseFinancialPlan, IncomeCents: 500000}), dispatcher, setter, v2)
	out, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "custo fixo 3000, metas 2000"})
	s.Require().NoError(err)
	s.True(out.Handled)
	s.True(interp.called)
	s.Equal(OnbPhaseFirstTx, setter.last())
	s.Contains(out.Reply, "primeiro lançamento")
	s.True(v2.cleared)
}

func (s *RunOnboardingTurnSuite) TestFirstTxClearsDraft() {
	interp := &fakeTurnInterpreter{resp: interfaces.LLMResponse{ToolCalls: []interfaces.ToolCall{{FunctionName: "record_transaction"}}}}
	dispatcher := &fakeToolDispatcher{results: map[string]OnboardingToolResult{
		"record_transaction": {Reply: "🏆 Boa!\n\n🎉 *Onboarding concluído!*", Advance: true, Terminal: true},
	}}
	setter := &fakePhaseSetter{}
	v2 := &fakeV2Session{}
	uc := s.newTurn(interp, newReader(OnboardingSnapshot{InProgress: true, Phase: OnbPhaseFirstTx}), dispatcher, setter, v2)
	out, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "gastei 35 no mercado"})
	s.Require().NoError(err)
	s.True(out.Handled)
	s.Contains(out.Reply, "Onboarding concluído")
	s.True(v2.cleared)
}

func (s *RunOnboardingTurnSuite) TestFirstTxNoTerminalNoClear() {
	interp := &fakeTurnInterpreter{resp: interfaces.LLMResponse{RawJSON: []byte("Me passa o valor? 😊")}}
	v2 := &fakeV2Session{}
	uc := s.newTurn(interp, newReader(OnboardingSnapshot{InProgress: true, Phase: OnbPhaseFirstTx}), &fakeToolDispatcher{results: map[string]OnboardingToolResult{}}, &fakePhaseSetter{}, v2)
	out, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "fiz uma compra"})
	s.Require().NoError(err)
	s.True(out.Handled)
	s.False(v2.cleared)
}

func (s *RunOnboardingTurnSuite) TestUnknownPhaseEmitsWelcome() {
	setter := &fakePhaseSetter{}
	uc := s.newTurn(&fakeTurnInterpreter{}, newReader(OnboardingSnapshot{InProgress: true, Phase: "methodology_1"}), &fakeToolDispatcher{}, setter, nil)
	out, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "oi"})
	s.Require().NoError(err)
	s.True(out.Handled)
	s.Contains(out.Reply, "Eu sou o *MeControla*")
	s.Equal(OnbPhaseObjective, setter.last())
}

func (s *RunOnboardingTurnSuite) TestInterpretErrorAtDataPhase() {
	interp := &fakeTurnInterpreter{err: errors.New("provider down")}
	uc := s.newTurn(interp, newReader(OnboardingSnapshot{InProgress: true, Phase: OnbPhaseObjective}), &fakeToolDispatcher{}, &fakePhaseSetter{}, nil)
	_, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "viagem"})
	s.Require().Error(err)
}

func (s *RunOnboardingTurnSuite) TestObjectiveSanitizesDoubleAsterisk() {
	interp := &fakeTurnInterpreter{resp: interfaces.LLMResponse{RawJSON: []byte("Seu **objetivo** foi anotado!")}}
	setter := &fakePhaseSetter{}
	uc := s.newTurn(interp, newReader(OnboardingSnapshot{InProgress: true, Phase: OnbPhaseObjective}), &fakeToolDispatcher{results: map[string]OnboardingToolResult{}}, setter, nil)
	out, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "quero comprar um carro"})
	s.Require().NoError(err)
	s.NotContains(out.Reply, "**")
	s.Contains(out.Reply, "*objetivo*")
}

func (s *RunOnboardingTurnSuite) TestCardsNegationVariants() {
	for _, neg := range []string{"não uso", "nao tenho", "n", "nenhum"} {
		setter := &fakePhaseSetter{}
		uc := s.newTurn(&fakeTurnInterpreter{}, newReader(OnboardingSnapshot{InProgress: true, Phase: OnbPhaseCards}), &fakeToolDispatcher{}, setter, &fakeV2Session{})
		out, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: neg})
		s.Require().NoError(err, "text: %q", neg)
		s.True(out.Handled, "text: %q", neg)
		s.Equal(OnbPhaseFinancialPlan, setter.last(), "text: %q", neg)
	}
}

func (s *RunOnboardingTurnSuite) TestNewRunOnboardingTurnNilV2SessionReturnsError() {
	_, err := NewRunOnboardingTurn(&fakeTurnInterpreter{}, &fakeStateReader{snapshots: []OnboardingSnapshot{{}}}, &fakeToolDispatcher{}, &fakePhaseSetter{}, 512, fake.NewProvider(), nil, nil)
	s.Require().Error(err)
	s.Contains(err.Error(), "v2session")
}

func (s *RunOnboardingTurnSuite) TestWelcomeReplyContainsAllFiveCategories() {
	uc := s.newTurn(&fakeTurnInterpreter{}, newReader(OnboardingSnapshot{InProgress: true, Phase: ""}), &fakeToolDispatcher{}, &fakePhaseSetter{}, nil)
	out, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "oi"})
	s.Require().NoError(err)
	for _, cat := range []string{"Custo Fixo", "Conhecimento", "Prazeres", "Metas", "Liberdade Financeira"} {
		s.Contains(out.Reply, cat, "categoria ausente: %s", cat)
	}
	s.Contains(out.Reply, "Pronto 😊")
	s.False(strings.Contains(out.Reply, "Faz sentido?"))
}

func (s *RunOnboardingTurnSuite) TestWelcomeSignalInProgressWithPhaseIsIdempotentNoOp() {
	interp := &fakeTurnInterpreter{}
	dispatcher := &fakeToolDispatcher{}
	setter := &fakePhaseSetter{}
	uc := s.newTurn(interp, newReader(OnboardingSnapshot{InProgress: true, Phase: OnbPhaseObjective}), dispatcher, setter, nil)
	out, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: services.OnboardingWelcomeSignal})
	s.Require().NoError(err)
	s.True(out.Handled)
	s.Equal("", out.Reply)
	s.False(interp.called)
	s.Equal(0, dispatcher.calls)
	s.Empty(setter.phases)
}

func (s *RunOnboardingTurnSuite) TestWelcomeSignalNotInProgressSwallowed() {
	interp := &fakeTurnInterpreter{}
	setter := &fakePhaseSetter{}
	uc := s.newTurn(interp, newReader(OnboardingSnapshot{InProgress: false}), &fakeToolDispatcher{}, setter, nil)
	out, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: services.OnboardingWelcomeSignal})
	s.Require().NoError(err)
	s.True(out.Handled)
	s.False(interp.called)
	s.Empty(setter.phases)
}

func (s *RunOnboardingTurnSuite) TestWelcomeSignalEmptyPhaseEmitsWelcome() {
	interp := &fakeTurnInterpreter{}
	setter := &fakePhaseSetter{}
	uc := s.newTurn(interp, newReader(OnboardingSnapshot{InProgress: true, Phase: ""}), &fakeToolDispatcher{}, setter, nil)
	out, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: services.OnboardingWelcomeSignal})
	s.Require().NoError(err)
	s.True(out.Handled)
	s.Equal(scriptWelcome, out.Reply)
	s.Equal(OnbPhaseObjective, setter.last())
	s.False(interp.called)
}

func (s *RunOnboardingTurnSuite) TestBudgetAutoSplitsSumEqualsIncome() {
	income := int64(300000)
	splits := buildAutoSplits(income)
	var total int64
	for _, sp := range splits {
		total += sp.AmountCents
	}
	s.Equal(income, total)
}
