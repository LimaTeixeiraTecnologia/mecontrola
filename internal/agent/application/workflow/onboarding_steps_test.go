package workflow

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type mockOnboardingInterpreter struct {
	welcomePrompt            string
	objectivePrompt          string
	budgetPrompt             string
	cardsPrompt              string
	categoriesPrompt         string
	valuesPrompt             string
	summaryPrompt            string
	retryPrompt              string
	dailyRedirectPrompt      string
	conclusionPrompt         string
	parseObjectiveResult     ParsedObjective
	parseBudgetResult        ParsedBudget
	parseCardsResult         ParsedCards
	parseSummaryResult       ParsedSummary
	parseValueResult         ParsedValue
	parseCategoriesConfirmed bool
	parseCategoriesErr       error
	lastSummaryState         SummaryState
}

func (m *mockOnboardingInterpreter) RenderWelcome(_ context.Context) string { return m.welcomePrompt }
func (m *mockOnboardingInterpreter) RenderObjective(_ context.Context) string {
	return m.objectivePrompt
}
func (m *mockOnboardingInterpreter) RenderBudget(_ context.Context) string { return m.budgetPrompt }
func (m *mockOnboardingInterpreter) RenderCards(_ context.Context, _ int) string {
	return m.cardsPrompt
}
func (m *mockOnboardingInterpreter) RenderCategories(_ context.Context) string {
	return m.categoriesPrompt
}
func (m *mockOnboardingInterpreter) RenderValues(_ context.Context, _ string) string {
	return m.valuesPrompt
}
func (m *mockOnboardingInterpreter) RenderSummary(_ context.Context, state SummaryState) string {
	m.lastSummaryState = state
	return m.summaryPrompt
}
func (m *mockOnboardingInterpreter) RenderRetry(_ context.Context, _ string) string {
	return m.retryPrompt
}
func (m *mockOnboardingInterpreter) RenderDailyRedirect(_ context.Context, _ string) string {
	return m.dailyRedirectPrompt
}
func (m *mockOnboardingInterpreter) RenderConclusion(_ context.Context) string {
	return m.conclusionPrompt
}
func (m *mockOnboardingInterpreter) RenderObjectiveSaved(_ context.Context) string {
	return "objective-saved"
}
func (m *mockOnboardingInterpreter) RenderBudgetSaved(_ context.Context, _ int64) string {
	return "budget-saved"
}
func (m *mockOnboardingInterpreter) RenderCardSaved(_ context.Context, _ string, _ int) string {
	return "card-saved"
}
func (m *mockOnboardingInterpreter) RenderValueSaved(_ context.Context, _ string, _ int64) string {
	return "value-saved"
}
func (m *mockOnboardingInterpreter) RenderCategoriesConfirmed(_ context.Context) string {
	return "categories-confirmed"
}
func (m *mockOnboardingInterpreter) RenderCategoriesClarify(_ context.Context) string {
	return "categories-clarify"
}
func (m *mockOnboardingInterpreter) RenderValuesMismatch(_ context.Context, _, _ int64) string {
	return "values-mismatch"
}
func (m *mockOnboardingInterpreter) ParseObjective(_ context.Context, _ string) (ParsedObjective, error) {
	return m.parseObjectiveResult, nil
}
func (m *mockOnboardingInterpreter) ParseBudget(_ context.Context, _ string) (ParsedBudget, error) {
	return m.parseBudgetResult, nil
}
func (m *mockOnboardingInterpreter) ParseCards(_ context.Context, _ string, _ int) (ParsedCards, error) {
	return m.parseCardsResult, nil
}
func (m *mockOnboardingInterpreter) ParseCategoriesConfirm(_ context.Context, _ string) (bool, error) {
	return m.parseCategoriesConfirmed && m.parseCategoriesErr == nil, m.parseCategoriesErr
}
func (m *mockOnboardingInterpreter) ParseValue(_ context.Context, _ string) (ParsedValue, error) {
	return m.parseValueResult, nil
}
func (m *mockOnboardingInterpreter) ParseSummary(_ context.Context, _ string) (ParsedSummary, error) {
	return m.parseSummaryResult, nil
}

type mockWelcomeMarker struct {
	alreadySent bool
	err         error
}

func (m *mockWelcomeMarker) Mark(_ context.Context, _ uuid.UUID) (bool, error) {
	return m.alreadySent, m.err
}

type mockObjectiveSaver struct {
	saved string
	err   error
}

func (m *mockObjectiveSaver) Save(_ context.Context, _ uuid.UUID, objective string) error {
	m.saved = objective
	return m.err
}

type mockIncomeSaver struct {
	savedCents int64
	err        error
}

func (m *mockIncomeSaver) Save(_ context.Context, _ uuid.UUID, incomeCents int64) error {
	m.savedCents = incomeCents
	return m.err
}

type mockCardSaver struct {
	saved []struct {
		nickname string
		dueDay   int
	}
	err error
}

func (m *mockCardSaver) Save(_ context.Context, _ uuid.UUID, nickname string, dueDay int) error {
	if m.err != nil {
		return m.err
	}
	m.saved = append(m.saved, struct {
		nickname string
		dueDay   int
	}{nickname: nickname, dueDay: dueDay})
	return nil
}

type mockSplitsSaver struct {
	values  map[string]int64
	applied bool
	err     error
}

func (m *mockSplitsSaver) Save(_ context.Context, _ uuid.UUID, values map[string]int64) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	m.values = values
	return m.applied, nil
}

type mockPhaseSetter struct {
	set string
	err error
}

func (m *mockPhaseSetter) Set(_ context.Context, _ uuid.UUID, phase string) error {
	m.set = phase
	return m.err
}

type mockSessionCompleter struct {
	completed bool
	err       error
}

func (m *mockSessionCompleter) Complete(_ context.Context, _ uuid.UUID) error {
	if m.err != nil {
		return m.err
	}
	m.completed = true
	return nil
}

type mockContextLoader struct {
	context OnboardingContext
	err     error
}

func (m *mockContextLoader) Load(_ context.Context, _ uuid.UUID) (OnboardingContext, error) {
	return m.context, m.err
}

type OnboardingStepsSuite struct {
	suite.Suite
	ctx         context.Context
	obs         *fake.Provider
	interpreter *mockOnboardingInterpreter
	welcome     *mockWelcomeMarker
	objective   *mockObjectiveSaver
	income      *mockIncomeSaver
	cards       *mockCardSaver
	splits      *mockSplitsSaver
	phase       *mockPhaseSetter
	completer   *mockSessionCompleter
	loader      *mockContextLoader
}

func TestOnboardingStepsSuite(t *testing.T) {
	suite.Run(t, new(OnboardingStepsSuite))
}

func (s *OnboardingStepsSuite) SetupTest() {
	s.ctx = context.Background()
	s.obs = fake.NewProvider()
	s.interpreter = &mockOnboardingInterpreter{
		welcomePrompt:            "welcome",
		objectivePrompt:          "objective",
		budgetPrompt:             "budget",
		cardsPrompt:              "cards",
		categoriesPrompt:         "categories",
		valuesPrompt:             "values",
		summaryPrompt:            "summary",
		retryPrompt:              "retry",
		dailyRedirectPrompt:      "redirect",
		conclusionPrompt:         "conclusion",
		parseCategoriesConfirmed: true,
	}
	s.welcome = &mockWelcomeMarker{}
	s.objective = &mockObjectiveSaver{}
	s.income = &mockIncomeSaver{}
	s.cards = &mockCardSaver{}
	s.splits = &mockSplitsSaver{applied: true}
	s.phase = &mockPhaseSetter{}
	s.completer = &mockSessionCompleter{}
	s.loader = &mockContextLoader{}
}

func (s *OnboardingStepsSuite) deps() onboardingDeps {
	return onboardingDeps{
		OnboardingDeps: OnboardingDeps{
			Interpreter:      s.interpreter,
			WelcomeMarker:    s.welcome,
			ObjectiveSaver:   s.objective,
			IncomeSaver:      s.income,
			CardSaver:        s.cards,
			SplitsSaver:      s.splits,
			PhaseSetter:      s.phase,
			ContextLoader:    s.loader,
			SessionCompleter: s.completer,
			O11y:             s.obs,
		},
		stepTotal: s.obs.Metrics().Counter(
			"onboarding_step_total",
			"Total de steps de onboarding executados por etapa e outcome",
			"1",
		),
	}
}

func (s *OnboardingStepsSuite) baseState() OnboardingState {
	return OnboardingState{UserID: uuid.MustParse("11111111-1111-1111-1111-111111111111")}
}

func (s *OnboardingStepsSuite) TestWelcomeStep_FirstEntry_Suspends() {
	step := newWelcomeStep(s.deps())
	out, err := step.Execute(s.ctx, s.baseState())
	s.NoError(err)
	s.Equal(platform.StepStatusSuspended, out.Status)
	s.Equal("welcome", out.Suspend.Prompt)
	s.Equal(AwaitingText, out.State.Awaiting)
}

func (s *OnboardingStepsSuite) TestWelcomeStep_Confirm_Advance() {
	step := newWelcomeStep(s.deps())
	state := s.baseState()
	state.Inbound = "sim"
	out, err := step.Execute(s.ctx, state)
	s.NoError(err)
	s.Equal(platform.StepStatusCompleted, out.Status)
}

func (s *OnboardingStepsSuite) TestWelcomeStep_DailyCommand_Redirects() {
	step := newWelcomeStep(s.deps())
	state := s.baseState()
	state.Inbound = "gastei 50 mercado"
	out, err := step.Execute(s.ctx, state)
	s.NoError(err)
	s.Equal(platform.StepStatusSuspended, out.Status)
	s.Equal("redirect", out.Suspend.Prompt)
}

func (s *OnboardingStepsSuite) TestWelcomeStep_MarkError_Fails() {
	s.welcome.err = errors.New("db unavailable")
	step := newWelcomeStep(s.deps())
	_, err := step.Execute(s.ctx, s.baseState())
	s.Error(err)
}

func (s *OnboardingStepsSuite) TestWelcomeStep_AlreadySent_Advance() {
	s.welcome.alreadySent = true
	step := newWelcomeStep(s.deps())
	out, err := step.Execute(s.ctx, s.baseState())
	s.NoError(err)
	s.Equal(platform.StepStatusCompleted, out.Status)
	s.Equal(valueobjects.PhaseObjective, out.State.Phase)
}

func (s *OnboardingStepsSuite) TestObjectiveStep_FirstEntry_Suspends() {
	step := newObjectiveStep(s.deps())
	out, err := step.Execute(s.ctx, s.baseState())
	s.NoError(err)
	s.Equal(platform.StepStatusSuspended, out.Status)
	s.Equal("objective", out.Suspend.Prompt)
}

func (s *OnboardingStepsSuite) TestObjectiveStep_Valid_Advance() {
	s.interpreter.parseObjectiveResult = ParsedObjective{Objective: "quitar dividas"}
	step := newObjectiveStep(s.deps())
	state := s.baseState()
	state.Inbound = "quitar dividas"
	out, err := step.Execute(s.ctx, state)
	s.NoError(err)
	s.Equal(platform.StepStatusCompleted, out.Status)
	s.Equal("quitar dividas", s.objective.saved)
}

func (s *OnboardingStepsSuite) TestObjectiveStep_Clarify_Retry() {
	s.interpreter.parseObjectiveResult = ParsedObjective{Ambiguous: true}
	step := newObjectiveStep(s.deps())
	state := s.baseState()
	state.Inbound = "nao sei"
	out, err := step.Execute(s.ctx, state)
	s.NoError(err)
	s.Equal(platform.StepStatusSuspended, out.Status)
	s.Equal("retry", out.Suspend.Prompt)
}

func (s *OnboardingStepsSuite) TestBudgetStep_FirstEntry_Suspends() {
	step := newBudgetStep(s.deps())
	out, err := step.Execute(s.ctx, s.baseState())
	s.NoError(err)
	s.Equal(platform.StepStatusSuspended, out.Status)
	s.Equal("budget", out.Suspend.Prompt)
}

func (s *OnboardingStepsSuite) TestBudgetStep_Valid_Advance() {
	s.interpreter.parseBudgetResult = ParsedBudget{IncomeCents: 500000}
	step := newBudgetStep(s.deps())
	state := s.baseState()
	state.Inbound = "5000"
	out, err := step.Execute(s.ctx, state)
	s.NoError(err)
	s.Equal(platform.StepStatusCompleted, out.Status)
	s.Equal(int64(500000), s.income.savedCents)
}

func (s *OnboardingStepsSuite) TestCardsStep_FirstEntry_Suspends() {
	step := newCardsStep(s.deps())
	out, err := step.Execute(s.ctx, s.baseState())
	s.NoError(err)
	s.Equal(platform.StepStatusSuspended, out.Status)
	s.Equal("cards", out.Suspend.Prompt)
}

func (s *OnboardingStepsSuite) TestCardsStep_Skip_Advance() {
	s.interpreter.parseCardsResult = ParsedCards{Skip: true}
	step := newCardsStep(s.deps())
	state := s.baseState()
	state.Inbound = "não uso"
	out, err := step.Execute(s.ctx, state)
	s.NoError(err)
	s.Equal(platform.StepStatusCompleted, out.Status)
}

func (s *OnboardingStepsSuite) TestCardsStep_SaveAndLoop() {
	s.interpreter.parseCardsResult = ParsedCards{Nickname: "Nubank", DueDay: 15}
	step := newCardsStep(s.deps())
	state := s.baseState()
	state.Inbound = "Nubank 15"
	out, err := step.Execute(s.ctx, state)
	s.NoError(err)
	s.Equal(platform.StepStatusSuspended, out.Status)
	s.Equal(1, out.State.CardLoop)
	s.Len(s.cards.saved, 1)
	s.Equal("Nubank", s.cards.saved[0].nickname)
	s.Equal(15, s.cards.saved[0].dueDay)
}

func (s *OnboardingStepsSuite) TestCardsStep_AddAnotherFirstLoop_AskDetails() {
	s.interpreter.parseCardsResult = ParsedCards{AddAnother: true}
	step := newCardsStep(s.deps())
	state := s.baseState()
	state.Inbound = "sim"
	out, err := step.Execute(s.ctx, state)
	s.NoError(err)
	s.Equal(platform.StepStatusSuspended, out.Status)
	s.Equal("cards", out.Suspend.Prompt)
	s.Equal(0, out.State.CardLoop)
	s.Empty(s.cards.saved)
}

func (s *OnboardingStepsSuite) TestCardsStep_AddAnotherNextLoop_AskDetails() {
	s.interpreter.parseCardsResult = ParsedCards{AddAnother: true}
	step := newCardsStep(s.deps())
	state := s.baseState()
	state.CardLoop = 1
	state.Inbound = "sim"
	out, err := step.Execute(s.ctx, state)
	s.NoError(err)
	s.Equal(platform.StepStatusSuspended, out.Status)
	s.Equal("cards", out.Suspend.Prompt)
	s.Equal(1, out.State.CardLoop)
	s.Empty(s.cards.saved)
}

func (s *OnboardingStepsSuite) TestCategoriesStep_FirstEntry_Suspends() {
	step := newCategoriesStep(s.deps())
	out, err := step.Execute(s.ctx, s.baseState())
	s.NoError(err)
	s.Equal(platform.StepStatusSuspended, out.Status)
	s.Equal("categories", out.Suspend.Prompt)
}

func (s *OnboardingStepsSuite) TestCategoriesStep_Confirm_Advance() {
	step := newCategoriesStep(s.deps())
	state := s.baseState()
	state.Inbound = "sim"
	out, err := step.Execute(s.ctx, state)
	s.NoError(err)
	s.Equal(platform.StepStatusCompleted, out.Status)
}

func (s *OnboardingStepsSuite) TestCategoriesStep_NonConfirm_AdvancesToValues() {
	s.interpreter.parseCategoriesConfirmed = false
	step := newCategoriesStep(s.deps())
	state := s.baseState()
	state.Inbound = "não entendi"
	out, err := step.Execute(s.ctx, state)
	s.NoError(err)
	s.Equal(platform.StepStatusCompleted, out.Status)
	s.Equal(valueobjects.PhaseValues, out.State.Phase)
}

func (s *OnboardingStepsSuite) TestCategoriesStep_DailyCommand_Deferred() {
	step := newCategoriesStep(s.deps())
	state := s.baseState()
	state.Inbound = "gastei 10 reais"
	out, err := step.Execute(s.ctx, state)
	s.NoError(err)
	s.Equal(platform.StepStatusSuspended, out.Status)
	s.Equal("redirect", out.Suspend.Prompt)
	s.Equal(valueobjects.PhaseCategories, out.State.Phase)
}

func (s *OnboardingStepsSuite) TestValuesStep_FirstEntry_Suspends() {
	s.loader.context.IncomeCents = 500000
	step := newValuesStep(s.deps())
	state := s.baseState()
	out, err := step.Execute(s.ctx, state)
	s.NoError(err)
	s.Equal(platform.StepStatusSuspended, out.Status)
	s.Equal("values", out.Suspend.Prompt)
	s.Equal("fixed_cost", pendingCategory(out.State.Values))
}

func (s *OnboardingStepsSuite) TestValuesStep_Complete_Advance() {
	s.interpreter.parseValueResult = ParsedValue{ValueCents: 100000}
	s.splits.applied = true
	s.loader.context.IncomeCents = 500000
	step := newValuesStep(s.deps())
	state := s.baseState()
	state.Values = map[string]int64{
		"knowledge":         50000,
		"pleasures":         75000,
		"goals":             100000,
		"financial_freedom": 175000,
	}
	state.Inbound = "1000"
	out, err := step.Execute(s.ctx, state)
	s.NoError(err)
	s.Equal(platform.StepStatusCompleted, out.Status)
	s.Equal(int64(100000), s.splits.values["fixed_cost"])
}

func (s *OnboardingStepsSuite) TestValuesStep_Mismatch_Clarify() {
	s.interpreter.parseValueResult = ParsedValue{ValueCents: 100000}
	s.loader.context.IncomeCents = 400000
	step := newValuesStep(s.deps())
	state := s.baseState()
	state.Values = map[string]int64{
		"knowledge":         50000,
		"pleasures":         75000,
		"goals":             100000,
		"financial_freedom": 175000,
	}
	state.Inbound = "1000"
	out, err := step.Execute(s.ctx, state)
	s.NoError(err)
	s.Equal(platform.StepStatusSuspended, out.Status)
	s.Equal("values-mismatch", out.Suspend.Prompt)
}

func (s *OnboardingStepsSuite) TestValuesStep_SaveError_Fail() {
	s.interpreter.parseValueResult = ParsedValue{ValueCents: 100000}
	s.splits.err = errors.New("save failed")
	s.loader.context.IncomeCents = 500000
	step := newValuesStep(s.deps())
	state := s.baseState()
	state.Values = map[string]int64{
		"knowledge":         50000,
		"pleasures":         75000,
		"goals":             100000,
		"financial_freedom": 175000,
	}
	state.Inbound = "1000"
	_, err := step.Execute(s.ctx, state)
	s.Error(err)
}

func (s *OnboardingStepsSuite) TestSummaryStep_FirstEntry_SuspendsWithAwaitingConfirm() {
	s.loader.context = OnboardingContext{
		Objective:   "viajar",
		IncomeCents: 500000,
	}
	step := newSummaryStep(s.deps())
	state := s.baseState()
	state.Values = map[string]int64{
		"fixed_cost":        200000,
		"knowledge":         50000,
		"pleasures":         75000,
		"goals":             100000,
		"financial_freedom": 75000,
	}
	out, err := step.Execute(s.ctx, state)
	s.NoError(err)
	s.Equal(platform.StepStatusSuspended, out.Status)
	s.Equal("summary", out.Suspend.Prompt)
	s.Equal(AwaitingConfirm, out.State.Awaiting)
}

func (s *OnboardingStepsSuite) TestSummaryStep_Confirm_Advance() {
	s.interpreter.parseSummaryResult = ParsedSummary{Confirm: true}
	s.loader.context = OnboardingContext{IncomeCents: 500000}
	step := newSummaryStep(s.deps())
	state := s.baseState()
	state.Inbound = "sim"
	out, err := step.Execute(s.ctx, state)
	s.NoError(err)
	s.Equal(platform.StepStatusCompleted, out.Status)
	s.Equal(valueobjects.PhaseConclusion, out.State.Phase)
}

func (s *OnboardingStepsSuite) TestSummaryStep_CorrectObjective_UpdatesAndReDisplays() {
	s.interpreter.parseSummaryResult = ParsedSummary{Correct: true, Target: CorrectionTargetObjective, NewValue: "comprar casa"}
	s.loader.context = OnboardingContext{Objective: "viajar"}
	step := newSummaryStep(s.deps())
	state := s.baseState()
	state.Inbound = "mudar objetivo para comprar casa"
	out, err := step.Execute(s.ctx, state)
	s.NoError(err)
	s.Equal(platform.StepStatusSuspended, out.Status)
	s.Equal("comprar casa", s.objective.saved)
}

func (s *OnboardingStepsSuite) TestSummaryStep_CorrectBudget_UpdatesAndReDisplays() {
	s.interpreter.parseSummaryResult = ParsedSummary{Correct: true, Target: CorrectionTargetBudget, NewValue: "6000"}
	s.loader.context = OnboardingContext{IncomeCents: 500000}
	step := newSummaryStep(s.deps())
	state := s.baseState()
	state.Inbound = "na verdade renda é 6000"
	out, err := step.Execute(s.ctx, state)
	s.NoError(err)
	s.Equal(platform.StepStatusSuspended, out.Status)
	s.Equal(int64(600000), s.income.savedCents)
}

func (s *OnboardingStepsSuite) TestSummaryStep_CorrectValues_UpdatesAndReDisplays() {
	s.interpreter.parseSummaryResult = ParsedSummary{Correct: true, Target: CorrectionTargetValues, NewValue: "2000,500,750,1000,750"}
	s.loader.context = OnboardingContext{IncomeCents: 500000}
	step := newSummaryStep(s.deps())
	state := s.baseState()
	state.Values = map[string]int64{
		"fixed_cost":        200000,
		"knowledge":         50000,
		"pleasures":         75000,
		"goals":             100000,
		"financial_freedom": 75000,
	}
	state.Inbound = "ajustar valores"
	out, err := step.Execute(s.ctx, state)
	s.NoError(err)
	s.Equal(platform.StepStatusSuspended, out.Status)
	s.Equal("summary", out.Suspend.Prompt)
	s.Equal(int64(200000), s.splits.values["fixed_cost"])
	s.Equal(int64(50000), s.splits.values["knowledge"])
	s.Equal(int64(75000), s.splits.values["pleasures"])
	s.Equal(int64(100000), s.splits.values["goals"])
	s.Equal(int64(75000), s.splits.values["financial_freedom"])
}

func (s *OnboardingStepsSuite) TestSummaryStep_CorrectValues_Malformed_Clarify() {
	s.interpreter.parseSummaryResult = ParsedSummary{Correct: true, Target: CorrectionTargetValues, NewValue: "1000"}
	s.loader.context = OnboardingContext{IncomeCents: 500000}
	step := newSummaryStep(s.deps())
	state := s.baseState()
	state.Values = map[string]int64{
		"fixed_cost":        200000,
		"knowledge":         50000,
		"pleasures":         75000,
		"goals":             100000,
		"financial_freedom": 75000,
	}
	state.Inbound = "ajustar valores para 1000"
	out, err := step.Execute(s.ctx, state)
	s.NoError(err)
	s.Equal(platform.StepStatusSuspended, out.Status)
	s.Equal("retry", out.Suspend.Prompt)
}

func (s *OnboardingStepsSuite) TestSummaryStep_CorrectCards_UpdatesAndReDisplays() {
	s.interpreter.parseSummaryResult = ParsedSummary{Correct: true, Target: CorrectionTargetCards, NewValue: "Nubank 20"}
	s.loader.context = OnboardingContext{Cards: []OnboardingCardState{{Name: "Nubank", DueDay: 10}}}
	step := newSummaryStep(s.deps())
	state := s.baseState()
	state.Inbound = "trocar cartão"
	out, err := step.Execute(s.ctx, state)
	s.NoError(err)
	s.Equal(platform.StepStatusSuspended, out.Status)
	s.Equal("summary", out.Suspend.Prompt)
	s.Len(s.cards.saved, 1)
	s.Equal("Nubank", s.cards.saved[0].nickname)
	s.Equal(20, s.cards.saved[0].dueDay)
}

func (s *OnboardingStepsSuite) TestSummaryStep_CorrectCards_Malformed_Clarify() {
	s.interpreter.parseSummaryResult = ParsedSummary{Correct: true, Target: CorrectionTargetCards, NewValue: "Nubank"}
	s.loader.context = OnboardingContext{Cards: []OnboardingCardState{{Name: "Nubank", DueDay: 10}}}
	step := newSummaryStep(s.deps())
	state := s.baseState()
	state.Inbound = "trocar cartão"
	out, err := step.Execute(s.ctx, state)
	s.NoError(err)
	s.Equal(platform.StepStatusSuspended, out.Status)
	s.Equal("retry", out.Suspend.Prompt)
}

func (s *OnboardingStepsSuite) TestPhaseSetter_CalledOnSuspendAndAdvance() {
	step := newObjectiveStep(s.deps())
	state := s.baseState()
	out, err := step.Execute(s.ctx, state)
	s.NoError(err)
	s.Equal(platform.StepStatusSuspended, out.Status)
	s.Equal("objective", s.phase.set)

	s.phase.set = ""
	s.interpreter.parseObjectiveResult = ParsedObjective{Objective: "quitar dividas"}
	state.Inbound = "quitar dividas"
	out, err = step.Execute(s.ctx, state)
	s.NoError(err)
	s.Equal(platform.StepStatusCompleted, out.Status)
	s.Equal("budget", s.phase.set)
}

func (s *OnboardingStepsSuite) TestPhaseSetter_Error_Fails() {
	s.phase.err = errors.New("phase set failed")
	step := newObjectiveStep(s.deps())
	state := s.baseState()
	_, err := step.Execute(s.ctx, state)
	s.Error(err)
}

func (s *OnboardingStepsSuite) TestSummaryStep_Ambiguous_Clarify() {
	s.interpreter.parseSummaryResult = ParsedSummary{Ambiguous: true}
	s.loader.context = OnboardingContext{IncomeCents: 500000}
	step := newSummaryStep(s.deps())
	state := s.baseState()
	state.Inbound = "sei la"
	out, err := step.Execute(s.ctx, state)
	s.NoError(err)
	s.Equal(platform.StepStatusSuspended, out.Status)
	s.Equal("retry", out.Suspend.Prompt)
}

func (s *OnboardingStepsSuite) TestSummaryStep_DailyCommand_Redirects() {
	s.interpreter.parseSummaryResult = ParsedSummary{DailyCommand: true}
	s.loader.context = OnboardingContext{IncomeCents: 500000}
	step := newSummaryStep(s.deps())
	state := s.baseState()
	state.Inbound = "gastei 50 mercado"
	out, err := step.Execute(s.ctx, state)
	s.NoError(err)
	s.Equal(platform.StepStatusSuspended, out.Status)
	s.Equal("redirect", out.Suspend.Prompt)
}

func (s *OnboardingStepsSuite) TestSummaryStep_RepromptOnceThenClarify() {
	s.interpreter.parseSummaryResult = ParsedSummary{}
	s.loader.context = OnboardingContext{IncomeCents: 500000}
	step := newSummaryStep(s.deps())
	state := s.baseState()
	state.Inbound = "hmm"
	out, err := step.Execute(s.ctx, state)
	s.NoError(err)
	s.Equal(platform.StepStatusSuspended, out.Status)
	s.Equal(1, out.State.RepromptCount)

	out.State.Inbound = "ainda não"
	out, err = step.Execute(s.ctx, out.State)
	s.NoError(err)
	s.Equal(platform.StepStatusSuspended, out.Status)
	s.Equal("retry", out.Suspend.Prompt)
}

func (s *OnboardingStepsSuite) TestSummaryStep_UsesContextLoaderForCanonicalData() {
	s.loader.context = OnboardingContext{
		Objective:   "viajar",
		IncomeCents: 500000,
		Cards:       []OnboardingCardState{{Name: "Nubank", DueDay: 15}},
	}
	step := newSummaryStep(s.deps())
	state := s.baseState()
	state.Values = map[string]int64{
		"fixed_cost":        200000,
		"knowledge":         50000,
		"pleasures":         75000,
		"goals":             100000,
		"financial_freedom": 75000,
	}
	out, err := step.Execute(s.ctx, state)
	s.NoError(err)
	s.Equal(platform.StepStatusSuspended, out.Status)
	s.Equal("viajar", s.interpreter.lastSummaryState.Objective)
	s.Equal(int64(500000), s.interpreter.lastSummaryState.IncomeCents)
	s.Equal(state.Values, s.interpreter.lastSummaryState.Values)
}

func (s *OnboardingStepsSuite) TestSummaryStep_ContextLoaderError_Fails() {
	s.loader.err = errors.New("context loader failed")
	step := newSummaryStep(s.deps())
	state := s.baseState()
	_, err := step.Execute(s.ctx, state)
	s.Error(err)
}

func (s *OnboardingStepsSuite) TestValuesStep_UsesContextLoaderIncome() {
	s.interpreter.parseValueResult = ParsedValue{ValueCents: 100000}
	s.splits.applied = true
	s.loader.context.IncomeCents = 500000
	step := newValuesStep(s.deps())
	state := s.baseState()
	state.Values = map[string]int64{
		"knowledge":         50000,
		"pleasures":         75000,
		"goals":             100000,
		"financial_freedom": 175000,
	}
	state.Inbound = "1000"
	out, err := step.Execute(s.ctx, state)
	s.NoError(err)
	s.Equal(platform.StepStatusCompleted, out.Status)
}

func (s *OnboardingStepsSuite) TestConclusionStep_CompletesSession() {
	step := newConclusionStep(s.deps())
	state := s.baseState()
	out, err := step.Execute(s.ctx, state)
	s.NoError(err)
	s.Equal(platform.StepStatusCompleted, out.Status)
	s.True(s.completer.completed)
	s.Equal("conclusion", out.Suspend.Prompt)
}

func (s *OnboardingStepsSuite) TestConclusionStep_CompleteError_Fail() {
	s.completer.err = errors.New("complete failed")
	step := newConclusionStep(s.deps())
	state := s.baseState()
	_, err := step.Execute(s.ctx, state)
	s.Error(err)
}
