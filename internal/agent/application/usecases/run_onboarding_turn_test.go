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
	onbusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	onbentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
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

type fakeHistoryGateway struct {
	turns       []onbentities.OnboardingTurn
	loadErr     error
	appendErr   error
	appendCalls int
	alreadySent bool
	markErr     error
	markCalls   int
}

func (g *fakeHistoryGateway) LoadTurns(_ context.Context, _ uuid.UUID) ([]onbentities.OnboardingTurn, error) {
	return g.turns, g.loadErr
}

func (g *fakeHistoryGateway) AppendTurn(_ context.Context, _ uuid.UUID, _, _ string) error {
	g.appendCalls++
	return g.appendErr
}

func (g *fakeHistoryGateway) MarkWelcomeSent(_ context.Context, _ uuid.UUID) (bool, error) {
	g.markCalls++
	return g.alreadySent, g.markErr
}

type fakeSplitSuggester struct {
	views  []onbusecases.SuggestBudgetSplitView
	err    error
	called bool
}

func (s *fakeSplitSuggester) Suggest(_ context.Context, _ uuid.UUID, _, _ string, _ int64) ([]onbusecases.SuggestBudgetSplitView, error) {
	s.called = true
	return s.views, s.err
}

var testDefaultSplit = []struct {
	slug string
	bp   int
}{
	{"expense.custo_fixo", 4000},
	{"expense.conhecimento", 1000},
	{"expense.prazeres", 1500},
	{"expense.metas", 2000},
	{"expense.liberdade_financeira", 1500},
}

func testBuildAutoSplits(incomeCents int64) []onboardingv2draft.SplitEntry {
	splits := make([]onboardingv2draft.SplitEntry, len(testDefaultSplit))
	var assigned int64
	for i, e := range testDefaultSplit {
		if i == len(testDefaultSplit)-1 {
			splits[i] = onboardingv2draft.SplitEntry{RootSlug: e.slug, AmountCents: incomeCents - assigned}
			continue
		}
		amt := incomeCents * int64(e.bp) / 10000
		splits[i] = onboardingv2draft.SplitEntry{RootSlug: e.slug, AmountCents: amt}
		assigned += amt
	}
	return splits
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
	return s.newTurnFull(interp, reader, dispatcher, phases, nil, nil, v2)
}

func (s *RunOnboardingTurnSuite) newTurnFull(interp IntentInterpreter, reader OnboardingStateReader, dispatcher OnboardingToolDispatcher, phases OnboardingPhaseSetter, history OnboardingHistoryGatewayIface, splitter BudgetSplitSuggesterIface, v2 OnboardingV2SessionGateway) *RunOnboardingTurn {
	if v2 == nil {
		v2 = &fakeV2Session{}
	}
	uc, err := NewRunOnboardingTurn(interp, reader, dispatcher, phases, 512, fake.NewProvider(), history, splitter, v2)
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
	s.True(interp.called)
}

func (s *RunOnboardingTurnSuite) TestEmitWelcomeIdempotentWhenAlreadySent() {
	history := &fakeHistoryGateway{alreadySent: true}
	setter := &fakePhaseSetter{}
	uc := s.newTurnFull(&fakeTurnInterpreter{}, newReader(OnboardingSnapshot{InProgress: true, Phase: ""}), &fakeToolDispatcher{}, setter, history, nil, nil)
	out, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "oi"})
	s.Require().NoError(err)
	s.True(out.Handled)
	s.Equal("", out.Reply)
	s.Equal(1, history.markCalls)
	s.Empty(setter.phases)
}

func (s *RunOnboardingTurnSuite) TestEmitWelcomeSetsPhaseWhenNotSent() {
	history := &fakeHistoryGateway{alreadySent: false}
	setter := &fakePhaseSetter{}
	uc := s.newTurnFull(&fakeTurnInterpreter{}, newReader(OnboardingSnapshot{InProgress: true, Phase: ""}), &fakeToolDispatcher{}, setter, history, nil, nil)
	out, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "oi"})
	s.Require().NoError(err)
	s.True(out.Handled)
	s.Contains(out.Reply, "Eu sou o *MeControla*")
	s.Equal(OnbPhaseObjective, setter.last())
}

func (s *RunOnboardingTurnSuite) TestObjectiveAdvancesToBudget() {
	interp := &fakeTurnInterpreter{resp: interfaces.LLMResponse{RawJSON: []byte(`{"action":"save_onboarding_objective","objective":"quitar dividas","objective_profile":"payoff_debt","reply":""}`)}}
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

func (s *RunOnboardingTurnSuite) TestBudgetAutoGeneratesSplitsViaSuggester() {
	interp := &fakeTurnInterpreter{resp: interfaces.LLMResponse{RawJSON: []byte(`{"action":"save_onboarding_income","income_cents":500000,"reply":""}`)}}

	dispatcher := &fakeToolDispatcher{results: map[string]OnboardingToolResult{
		ToolSaveOnboardingIncome: {Reply: "💰 Orçamento salvo!", Advance: true},
	}}
	setter := &fakePhaseSetter{}
	v2 := &fakeV2Session{}
	snapAfter := OnboardingSnapshot{InProgress: true, Phase: OnbPhaseBudget, IncomeCents: 500000}
	suggester := &fakeSplitSuggester{views: []onbusecases.SuggestBudgetSplitView{
		{RootSlug: "expense.custo_fixo", PlannedCents: 200000},
		{RootSlug: "expense.conhecimento", PlannedCents: 50000},
		{RootSlug: "expense.prazeres", PlannedCents: 75000},
		{RootSlug: "expense.metas", PlannedCents: 100000},
		{RootSlug: "expense.liberdade_financeira", PlannedCents: 75000},
	}}
	uc := s.newTurnFull(interp,
		newReader(OnboardingSnapshot{InProgress: true, Phase: OnbPhaseBudget}, snapAfter),
		dispatcher, setter, nil, suggester, v2)
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
	s.True(suggester.called)
}

func (s *RunOnboardingTurnSuite) TestBudgetWithNoSuggesterProducesNoAutoSplits() {
	interp := &fakeTurnInterpreter{resp: interfaces.LLMResponse{RawJSON: []byte(`{"action":"save_onboarding_income","income_cents":500000,"reply":""}`)}}

	dispatcher := &fakeToolDispatcher{results: map[string]OnboardingToolResult{
		ToolSaveOnboardingIncome: {Reply: "💰 Orçamento salvo!", Advance: true},
	}}
	setter := &fakePhaseSetter{}
	v2 := &fakeV2Session{}
	snapAfter := OnboardingSnapshot{InProgress: true, Phase: OnbPhaseBudget, IncomeCents: 500000}
	uc := s.newTurnFull(interp,
		newReader(OnboardingSnapshot{InProgress: true, Phase: OnbPhaseBudget}, snapAfter),
		dispatcher, setter, nil, nil, v2)
	out, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "ganho 5000"})
	s.Require().NoError(err)
	s.True(out.Handled)
	s.Equal(OnbPhaseCards, setter.last())
	s.NotNil(v2.saved)
	s.Len(v2.saved.Splits(), 0)
}

func (s *RunOnboardingTurnSuite) TestCardsNegationSkipsToFinancialPlan() {
	setter := &fakePhaseSetter{}
	autoSplits := testBuildAutoSplits(500000)
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
	interp := &fakeTurnInterpreter{resp: interfaces.LLMResponse{RawJSON: []byte(`{"action":"save_onboarding_card","cards":[{"nickname":"Nubank","closing_day":13},{"nickname":"Inter","closing_day":5}],"reply":""}`)}}

	dispatcher := &fakeToolDispatcher{results: map[string]OnboardingToolResult{
		ToolSaveOnboardingCard: {Reply: "💳 Cartão salvo!", Advance: true},
	}}
	setter := &fakePhaseSetter{}
	autoSplits := testBuildAutoSplits(500000)
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

func (s *RunOnboardingTurnSuite) TestCardsAdvanceSummaryIncludesJustAddedCard() {
	interp := &fakeTurnInterpreter{resp: interfaces.LLMResponse{RawJSON: []byte(`{"action":"save_onboarding_card","cards":[{"nickname":"Nubank","closing_day":13}],"reply":""}`)}}

	dispatcher := &fakeToolDispatcher{results: map[string]OnboardingToolResult{
		ToolSaveOnboardingCard: {Reply: "💳 Cartão salvo!", Advance: true},
	}}
	setter := &fakePhaseSetter{}
	autoSplits := testBuildAutoSplits(500000)
	draft := onboardingv2draft.New().WithAutoSplits(autoSplits)
	v2 := &fakeV2Session{draft: draft, found: true}
	snapBefore := OnboardingSnapshot{InProgress: true, Phase: OnbPhaseCards, IncomeCents: 500000}
	snapAfter := OnboardingSnapshot{InProgress: true, Phase: OnbPhaseCards, IncomeCents: 500000, Cards: []OnboardingSnapshotCard{{Name: "Nubank", ClosingDay: 13}}}
	uc := s.newTurn(interp, newReader(snapBefore, snapAfter), dispatcher, setter, v2)
	out, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "Nubank 13"})
	s.Require().NoError(err)
	s.True(out.Handled)
	s.Equal(OnbPhaseFinancialPlan, setter.last())
	s.Contains(out.Reply, "Etapa 4/4")
	s.Contains(out.Reply, "Nubank")
}

func (s *RunOnboardingTurnSuite) TestFinancialPlanConfirmUsesAutoSplits() {
	dispatcher := &fakeToolDispatcher{results: map[string]OnboardingToolResult{
		ToolSaveOnboardingBudgetSplits: {Reply: "✅ Plano salvo!", Advance: true},
	}}
	setter := &fakePhaseSetter{}
	autoSplits := testBuildAutoSplits(500000)
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
	interp := &fakeTurnInterpreter{resp: interfaces.LLMResponse{RawJSON: []byte(`{"action":"save_onboarding_budget_splits","allocations":[{"root_slug":"expense.custo_fixo","amount_cents":300000},{"root_slug":"expense.metas","amount_cents":200000}],"reply":""}`)}}

	dispatcher := &fakeToolDispatcher{results: map[string]OnboardingToolResult{
		ToolSaveOnboardingBudgetSplits: {Reply: "✅ Distribuição ajustada!", Advance: true},
	}}
	setter := &fakePhaseSetter{}
	autoSplits := testBuildAutoSplits(500000)
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
	interp := &fakeTurnInterpreter{resp: interfaces.LLMResponse{RawJSON: []byte(`{"action":"record_transaction","direction":"outcome","amount_cents":3500,"merchant":"mercado","category_hint":"","reply":""}`)}}

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

func (s *RunOnboardingTurnSuite) TestLLMErrorDoesNotPersistPhaseTransition() {
	interp := &fakeTurnInterpreter{err: errors.New("llm unavailable")}
	setter := &fakePhaseSetter{}
	v2 := &fakeV2Session{}

	scenarios := []struct {
		name  string
		phase string
	}{
		{"objective phase", OnbPhaseObjective},
		{"budget phase", OnbPhaseBudget},
		{"cards phase", OnbPhaseCards},
		{"financial_plan phase", OnbPhaseFinancialPlan},
		{"first_tx phase", OnbPhaseFirstTx},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			setter.phases = nil
			v2.saved = nil
			v2.cleared = false

			var snap OnboardingSnapshot
			if scenario.phase == OnbPhaseBudget {
				snap = OnboardingSnapshot{InProgress: true, Phase: scenario.phase, IncomeCents: 500000}
			} else {
				snap = OnboardingSnapshot{InProgress: true, Phase: scenario.phase}
			}

			uc := s.newTurnFull(interp, newReader(snap, snap), &fakeToolDispatcher{}, setter, nil, nil, v2)
			var text string
			switch scenario.phase {
			case OnbPhaseCards:
				text = "nubank 10"
			case OnbPhaseFinancialPlan:
				text = "quero ajustar 3000 no custo fixo"
			default:
				text = "texto de teste"
			}
			_, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: text})

			s.Require().Error(err)
			s.Empty(setter.phases, "nenhuma transicao de fase deve ser persistida em erro de LLM")
			s.Nil(v2.saved, "nenhum draft deve ser salvo em erro de LLM")
			s.False(v2.cleared, "draft nao deve ser limpo em erro de LLM")
		})
	}
}

func (s *RunOnboardingTurnSuite) TestLLMErrorDoesNotCallCompleteOnboardingSession() {
	interp := &fakeTurnInterpreter{err: errors.New("llm down")}
	setter := &fakePhaseSetter{}
	v2 := &fakeV2Session{}
	dispatcher := &fakeToolDispatcher{}

	uc := s.newTurnFull(interp, newReader(OnboardingSnapshot{InProgress: true, Phase: OnbPhaseFirstTx}), dispatcher, setter, nil, nil, v2)
	_, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "gastei 50"})

	s.Require().Error(err)
	s.Zero(dispatcher.calls, "dispatcher nao deve ser chamado em erro de LLM")
	s.Empty(setter.phases)
	s.False(v2.cleared)
}

func (s *RunOnboardingTurnSuite) TestObjectiveFallbackOnInvalidJSON() {
	interp := &fakeTurnInterpreter{resp: interfaces.LLMResponse{RawJSON: []byte("nao e json valido")}}
	setter := &fakePhaseSetter{}
	uc := s.newTurn(interp, newReader(OnboardingSnapshot{InProgress: true, Phase: OnbPhaseObjective}), &fakeToolDispatcher{results: map[string]OnboardingToolResult{}}, setter, nil)
	out, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "quero comprar um carro"})
	s.Require().NoError(err)
	s.True(out.Handled)
	s.NotEmpty(out.Reply)
	s.Empty(setter.phases)
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
	_, err := NewRunOnboardingTurn(&fakeTurnInterpreter{}, &fakeStateReader{snapshots: []OnboardingSnapshot{{}}}, &fakeToolDispatcher{}, &fakePhaseSetter{}, 512, fake.NewProvider(), nil, nil, nil)
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
	s.True(interp.called)
}

func (s *RunOnboardingTurnSuite) TestHistoryGatewayAppendCalledAfterPhase() {
	interp := &fakeTurnInterpreter{resp: interfaces.LLMResponse{RawJSON: []byte("Que objetivo legal!")}}
	history := &fakeHistoryGateway{}
	uc := s.newTurnFull(interp, newReader(OnboardingSnapshot{InProgress: true, Phase: OnbPhaseObjective}), &fakeToolDispatcher{results: map[string]OnboardingToolResult{}}, &fakePhaseSetter{}, history, nil, nil)
	_, err := uc.Execute(s.ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "quitar dividas"})
	s.Require().NoError(err)
	s.Equal(1, history.appendCalls)
}
