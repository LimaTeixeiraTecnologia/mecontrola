package workflow

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type OnboardingWorkflowSuite struct {
	suite.Suite
	obs         *fake.Provider
	store       *fakeStore
	interpreter *mockOnboardingInterpreter
	objective   *mockObjectiveSaver
	income      *mockIncomeSaver
	cards       *mockCardSaver
	splits      *mockSplitsSaver
	completer   *mockSessionCompleter
	loader      *mockContextLoader
}

func TestOnboardingWorkflowSuite(t *testing.T) {
	suite.Run(t, new(OnboardingWorkflowSuite))
}

func (s *OnboardingWorkflowSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.store = newFakeStore()
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
	s.objective = &mockObjectiveSaver{}
	s.income = &mockIncomeSaver{}
	s.cards = &mockCardSaver{}
	s.splits = &mockSplitsSaver{applied: true}
	s.completer = &mockSessionCompleter{}
	s.loader = &mockContextLoader{}
}

func (s *OnboardingWorkflowSuite) deps() OnboardingDeps {
	return OnboardingDeps{
		Interpreter:      s.interpreter,
		WelcomeMarker:    &mockWelcomeMarker{},
		ObjectiveSaver:   s.objective,
		IncomeSaver:      s.income,
		CardSaver:        s.cards,
		SplitsSaver:      s.splits,
		PhaseSetter:      &mockPhaseSetter{},
		ContextLoader:    s.loader,
		SessionCompleter: s.completer,
		O11y:             s.obs,
	}
}

func (s *OnboardingWorkflowSuite) initialState() OnboardingState {
	return OnboardingState{UserID: uuid.MustParse("22222222-2222-2222-2222-222222222222")}
}

func (s *OnboardingWorkflowSuite) resumePayload(text, messageID string) []byte {
	b, _ := json.Marshal(map[string]any{"inbound": text, "message_id": messageID})
	return b
}

func (s *OnboardingWorkflowSuite) TestWorkflow_WelcomeToObjective() {
	def := BuildOnboardingDefinition(s.deps())
	eng := platform.NewEngine[OnboardingState](s.store, s.obs)
	res, err := eng.Start(s.ctx(), def, "user-1", s.initialState())
	s.NoError(err)
	s.Equal(platform.RunStatusSuspended, res.Status)
	s.Equal("welcome", res.Suspend.Prompt)

	res, err = eng.Resume(s.ctx(), def, "user-1", s.resumePayload("sim", "m1"))
	s.NoError(err)
	s.Equal(platform.RunStatusSuspended, res.Status)
	s.Equal("objective", res.Suspend.Prompt)
	s.Equal(valueobjects.PhaseObjective, res.State.Phase)
}

func (s *OnboardingWorkflowSuite) TestWorkflow_ObjectiveToBudget() {
	s.interpreter.parseObjectiveResult = ParsedObjective{Objective: "viajar"}
	def := BuildOnboardingDefinition(s.deps())
	eng := platform.NewEngine[OnboardingState](s.store, s.obs)
	_, _ = eng.Start(s.ctx(), def, "user-1", s.initialState())
	_, _ = eng.Resume(s.ctx(), def, "user-1", s.resumePayload("sim", "m1"))
	res, err := eng.Resume(s.ctx(), def, "user-1", s.resumePayload("viajar", "m2"))
	s.NoError(err)
	s.Equal(platform.RunStatusSuspended, res.Status)
	s.Equal("objective-saved\n\nbudget", res.Suspend.Prompt)
	s.Equal("viajar", s.objective.saved)
}

func (s *OnboardingWorkflowSuite) TestWorkflow_BudgetToCards() {
	s.interpreter.parseObjectiveResult = ParsedObjective{Objective: "viajar"}
	s.interpreter.parseBudgetResult = ParsedBudget{IncomeCents: 400000}
	s.loader.context = OnboardingContext{IncomeCents: 400000}
	def := BuildOnboardingDefinition(s.deps())
	eng := platform.NewEngine[OnboardingState](s.store, s.obs)
	_, _ = eng.Start(s.ctx(), def, "user-1", s.initialState())
	_, _ = eng.Resume(s.ctx(), def, "user-1", s.resumePayload("sim", "m1"))
	_, _ = eng.Resume(s.ctx(), def, "user-1", s.resumePayload("viajar", "m2"))
	res, err := eng.Resume(s.ctx(), def, "user-1", s.resumePayload("4000", "m3"))
	s.NoError(err)
	s.Equal(platform.RunStatusSuspended, res.Status)
	s.Equal("budget-saved\n\ncards", res.Suspend.Prompt)
}

func (s *OnboardingWorkflowSuite) TestWorkflow_CardsSkipToCategories() {
	s.interpreter.parseObjectiveResult = ParsedObjective{Objective: "viajar"}
	s.interpreter.parseBudgetResult = ParsedBudget{IncomeCents: 400000}
	s.interpreter.parseCardsResult = ParsedCards{Skip: true}
	def := BuildOnboardingDefinition(s.deps())
	eng := platform.NewEngine[OnboardingState](s.store, s.obs)
	_, _ = eng.Start(s.ctx(), def, "user-1", s.initialState())
	_, _ = eng.Resume(s.ctx(), def, "user-1", s.resumePayload("sim", "m1"))
	_, _ = eng.Resume(s.ctx(), def, "user-1", s.resumePayload("viajar", "m2"))
	_, _ = eng.Resume(s.ctx(), def, "user-1", s.resumePayload("4000", "m3"))
	res, err := eng.Resume(s.ctx(), def, "user-1", s.resumePayload("não uso", "m4"))
	s.NoError(err)
	s.Equal(platform.RunStatusSuspended, res.Status)
	s.Equal("categories", res.Suspend.Prompt)
}

func (s *OnboardingWorkflowSuite) TestWorkflow_CategoriesToValues() {
	s.interpreter.parseObjectiveResult = ParsedObjective{Objective: "viajar"}
	s.interpreter.parseBudgetResult = ParsedBudget{IncomeCents: 500000}
	s.interpreter.parseCardsResult = ParsedCards{Skip: true}
	s.loader.context = OnboardingContext{IncomeCents: 500000}
	def := BuildOnboardingDefinition(s.deps())
	eng := platform.NewEngine[OnboardingState](s.store, s.obs)
	_, _ = eng.Start(s.ctx(), def, "user-1", s.initialState())
	_, _ = eng.Resume(s.ctx(), def, "user-1", s.resumePayload("sim", "m1"))
	_, _ = eng.Resume(s.ctx(), def, "user-1", s.resumePayload("viajar", "m2"))
	_, _ = eng.Resume(s.ctx(), def, "user-1", s.resumePayload("5000", "m3"))
	_, _ = eng.Resume(s.ctx(), def, "user-1", s.resumePayload("não uso", "m4"))
	res, err := eng.Resume(s.ctx(), def, "user-1", s.resumePayload("ok", "m5"))
	s.NoError(err)
	s.Equal(platform.RunStatusSuspended, res.Status)
	s.Equal("categories-confirmed\n\nvalues", res.Suspend.Prompt)
	s.Equal(valueobjects.PhaseValues, res.State.Phase)
}

func (s *OnboardingWorkflowSuite) TestWorkflow_CardLoop() {
	s.interpreter.parseObjectiveResult = ParsedObjective{Objective: "viajar"}
	s.interpreter.parseBudgetResult = ParsedBudget{IncomeCents: 500000}
	def := BuildOnboardingDefinition(s.deps())
	eng := platform.NewEngine[OnboardingState](s.store, s.obs)
	_, _ = eng.Start(s.ctx(), def, "user-1", s.initialState())
	_, _ = eng.Resume(s.ctx(), def, "user-1", s.resumePayload("sim", "m1"))
	_, _ = eng.Resume(s.ctx(), def, "user-1", s.resumePayload("viajar", "m2"))
	_, _ = eng.Resume(s.ctx(), def, "user-1", s.resumePayload("5000", "m3"))

	s.interpreter.parseCardsResult = ParsedCards{Nickname: "Nubank", DueDay: 15}
	res, err := eng.Resume(s.ctx(), def, "user-1", s.resumePayload("Nubank 15", "m4"))
	s.NoError(err)
	s.Equal(platform.RunStatusSuspended, res.Status)
	s.Equal(1, res.State.CardLoop)
	s.Equal("card-saved\n\ncards", res.Suspend.Prompt)

	s.interpreter.parseCardsResult = ParsedCards{Nickname: "Inter", DueDay: 20}
	res, err = eng.Resume(s.ctx(), def, "user-1", s.resumePayload("Inter 20", "m5"))
	s.NoError(err)
	s.Equal(platform.RunStatusSuspended, res.Status)
	s.Equal(2, res.State.CardLoop)

	s.interpreter.parseCardsResult = ParsedCards{Skip: true}
	res, err = eng.Resume(s.ctx(), def, "user-1", s.resumePayload("não", "m6"))
	s.NoError(err)
	s.Equal(platform.RunStatusSuspended, res.Status)
	s.Equal("categories", res.Suspend.Prompt)
	s.Len(s.cards.saved, 2)
}

func (s *OnboardingWorkflowSuite) TestWorkflow_CategoriesNotConfirmed_ClarifiesAndProceeds() {
	s.interpreter.parseObjectiveResult = ParsedObjective{Objective: "viajar"}
	s.interpreter.parseBudgetResult = ParsedBudget{IncomeCents: 500000}
	s.interpreter.parseCardsResult = ParsedCards{Skip: true}
	s.interpreter.parseCategoriesConfirmed = false
	s.loader.context = OnboardingContext{IncomeCents: 500000}
	def := BuildOnboardingDefinition(s.deps())
	eng := platform.NewEngine[OnboardingState](s.store, s.obs)
	_, _ = eng.Start(s.ctx(), def, "user-1", s.initialState())
	_, _ = eng.Resume(s.ctx(), def, "user-1", s.resumePayload("sim", "m1"))
	_, _ = eng.Resume(s.ctx(), def, "user-1", s.resumePayload("viajar", "m2"))
	_, _ = eng.Resume(s.ctx(), def, "user-1", s.resumePayload("5000", "m3"))
	_, _ = eng.Resume(s.ctx(), def, "user-1", s.resumePayload("não uso", "m4"))
	res, err := eng.Resume(s.ctx(), def, "user-1", s.resumePayload("o que é isso?", "m5"))
	s.NoError(err)
	s.Equal(platform.RunStatusSuspended, res.Status)
	s.Equal("categories-clarify\n\nvalues", res.Suspend.Prompt)
	s.Equal(valueobjects.PhaseValues, res.State.Phase)
}

func (s *OnboardingWorkflowSuite) TestWorkflow_ValuesMismatch_ExplainsAndStays() {
	s.interpreter.parseObjectiveResult = ParsedObjective{Objective: "viajar"}
	s.interpreter.parseBudgetResult = ParsedBudget{IncomeCents: 500000}
	s.interpreter.parseCardsResult = ParsedCards{Skip: true}
	s.loader.context = OnboardingContext{IncomeCents: 500000}
	def := BuildOnboardingDefinition(s.deps())
	eng := platform.NewEngine[OnboardingState](s.store, s.obs)
	_, _ = eng.Start(s.ctx(), def, "user-1", s.initialState())
	_, _ = eng.Resume(s.ctx(), def, "user-1", s.resumePayload("sim", "m1"))
	_, _ = eng.Resume(s.ctx(), def, "user-1", s.resumePayload("viajar", "m2"))
	_, _ = eng.Resume(s.ctx(), def, "user-1", s.resumePayload("5000", "m3"))
	_, _ = eng.Resume(s.ctx(), def, "user-1", s.resumePayload("não uso", "m4"))
	_, _ = eng.Resume(s.ctx(), def, "user-1", s.resumePayload("ok", "m5"))

	amounts := []int64{100000, 100000, 100000, 100000, 50000}
	var res platform.RunResult[OnboardingState]
	var err error
	for idx, amount := range amounts {
		s.interpreter.parseValueResult = ParsedValue{ValueCents: amount}
		res, err = eng.Resume(s.ctx(), def, "user-1", s.resumePayload("v", "vm"+string(rune('a'+idx))))
		s.NoError(err)
	}
	s.Equal(platform.RunStatusSuspended, res.Status)
	s.Equal(valueobjects.PhaseValues, res.State.Phase)
	s.Equal("values-mismatch", res.Suspend.Prompt)
	s.False(s.completer.completed)
}

func (s *OnboardingWorkflowSuite) TestWorkflow_ValuesToSummary_ConfirmsEachAndAdvances() {
	s.interpreter.parseObjectiveResult = ParsedObjective{Objective: "viajar"}
	s.interpreter.parseBudgetResult = ParsedBudget{IncomeCents: 500000}
	s.interpreter.parseCardsResult = ParsedCards{Skip: true}
	s.loader.context = OnboardingContext{IncomeCents: 500000}
	def := BuildOnboardingDefinition(s.deps())
	eng := platform.NewEngine[OnboardingState](s.store, s.obs)
	_, _ = eng.Start(s.ctx(), def, "user-1", s.initialState())
	_, _ = eng.Resume(s.ctx(), def, "user-1", s.resumePayload("sim", "m1"))
	_, _ = eng.Resume(s.ctx(), def, "user-1", s.resumePayload("viajar", "m2"))
	_, _ = eng.Resume(s.ctx(), def, "user-1", s.resumePayload("5000", "m3"))
	_, _ = eng.Resume(s.ctx(), def, "user-1", s.resumePayload("não uso", "m4"))
	_, _ = eng.Resume(s.ctx(), def, "user-1", s.resumePayload("ok", "m5"))

	var res platform.RunResult[OnboardingState]
	var err error
	for idx := range 5 {
		s.interpreter.parseValueResult = ParsedValue{ValueCents: 100000}
		res, err = eng.Resume(s.ctx(), def, "user-1", s.resumePayload("1000", "vs"+string(rune('a'+idx))))
		s.NoError(err)
	}
	s.Equal(platform.RunStatusSuspended, res.Status)
	s.Equal(valueobjects.PhaseSummary, res.State.Phase)
	s.Contains(res.Suspend.Prompt, "value-saved")
	s.Contains(res.Suspend.Prompt, "summary")
}

func (s *OnboardingWorkflowSuite) ctx() context.Context {
	return context.Background()
}

type fakeStore struct {
	mu    sync.RWMutex
	snaps map[string]platform.Snapshot
	steps []platform.StepRecord
}

func newFakeStore() *fakeStore {
	return &fakeStore{snaps: make(map[string]platform.Snapshot)}
}

func (f *fakeStore) storeKey(workflow, key string) string { return workflow + "::" + key }

func (f *fakeStore) Insert(_ context.Context, snap platform.Snapshot) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.snaps[f.storeKey(snap.Workflow, snap.CorrelationKey)] = snap
	return nil
}

func (f *fakeStore) Load(_ context.Context, workflow, key string) (platform.Snapshot, bool, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	snap, ok := f.snaps[f.storeKey(workflow, key)]
	return snap, ok, nil
}

func (f *fakeStore) Save(_ context.Context, snap platform.Snapshot, expectedVersion int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	existing, ok := f.snaps[f.storeKey(snap.Workflow, snap.CorrelationKey)]
	if ok && existing.Version != expectedVersion {
		return platform.ErrVersionConflict
	}
	f.snaps[f.storeKey(snap.Workflow, snap.CorrelationKey)] = snap
	return nil
}

func (f *fakeStore) AppendStep(_ context.Context, rec platform.StepRecord) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.steps = append(f.steps, rec)
	return nil
}

func (f *fakeStore) ListSuspended(_ context.Context, workflow string, updatedBefore time.Time, limit int) ([]platform.Snapshot, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	var result []platform.Snapshot
	for _, snap := range f.snaps {
		if snap.Workflow != workflow {
			continue
		}
		if snap.Status != platform.RunStatusSuspended {
			continue
		}
		if !snap.UpdatedAt.Before(updatedBefore) {
			continue
		}
		result = append(result, snap)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result, nil
}

func (f *fakeStore) DeleteCompleted(_ context.Context, _ time.Duration, _ int) (int64, error) {
	return 0, nil
}
