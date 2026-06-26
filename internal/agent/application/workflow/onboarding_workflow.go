package workflow

import (
	"context"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type OnboardingInterpreter interface {
	RenderWelcome(ctx context.Context) string
	RenderObjective(ctx context.Context) string
	RenderBudget(ctx context.Context) string
	RenderCards(ctx context.Context, loop int) string
	RenderCategories(ctx context.Context) string
	RenderValues(ctx context.Context, pending string) string
	RenderSummary(ctx context.Context, state SummaryState) string
	RenderRetry(ctx context.Context, phase string) string
	RenderDailyRedirect(ctx context.Context, phase string) string
	RenderConclusion(ctx context.Context) string
	ParseObjective(ctx context.Context, text string) (ParsedObjective, error)
	ParseBudget(ctx context.Context, text string) (ParsedBudget, error)
	ParseCards(ctx context.Context, text string, loop int) (ParsedCards, error)
	ParseCategoriesConfirm(ctx context.Context, text string) (bool, error)
	ParseValue(ctx context.Context, text string) (ParsedValue, error)
	ParseSummary(ctx context.Context, text string) (ParsedSummary, error)
}

type ParsedValue struct {
	ValueCents   int64
	Ambiguous    bool
	DailyCommand bool
}

type SummaryState struct {
	Objective   string
	IncomeCents int64
	Values      map[string]int64
}

type OnboardingContext struct {
	Objective   string
	IncomeCents int64
	Cards       []OnboardingCardState
}

type ContextLoader interface {
	Load(ctx context.Context, userID uuid.UUID) (OnboardingContext, error)
}

type SessionCompleter interface {
	Complete(ctx context.Context, userID uuid.UUID) error
}

type WelcomeMarker interface {
	Mark(ctx context.Context, userID uuid.UUID) (alreadySent bool, err error)
}

type ObjectiveSaver interface {
	Save(ctx context.Context, userID uuid.UUID, objective string) error
}

type IncomeSaver interface {
	Save(ctx context.Context, userID uuid.UUID, incomeCents int64) error
}

type CardSaver interface {
	Save(ctx context.Context, userID uuid.UUID, nickname string, dueDay int) error
}

type SplitsSaver interface {
	Save(ctx context.Context, userID uuid.UUID, values map[string]int64) (bool, error)
}

type PhaseSetter interface {
	Set(ctx context.Context, userID uuid.UUID, phase string) error
}

type HistoryGateway interface {
	AppendTurn(ctx context.Context, userID uuid.UUID, userMsg, assistantReply string) error
}

type OnboardingDeps struct {
	Interpreter   OnboardingInterpreter
	WelcomeMarker WelcomeMarker
	ObjectiveSaver
	IncomeSaver
	CardSaver
	SplitsSaver
	PhaseSetter
	ContextLoader    ContextLoader
	SessionCompleter SessionCompleter
	HistoryGateway   HistoryGateway
	O11y             observability.Observability
}

type onboardingDeps struct {
	OnboardingDeps
	stepTotal observability.Counter
}

func BuildOnboardingDefinition(d OnboardingDeps) platform.Definition[OnboardingState] {
	deps := onboardingDeps{
		OnboardingDeps: d,
		stepTotal: d.O11y.Metrics().Counter(
			"onboarding_step_total",
			"Total de steps de onboarding executados por etapa e outcome",
			"1",
		),
	}
	return platform.Definition[OnboardingState]{
		ID:          "onboarding",
		Durable:     true,
		MaxAttempts: 1,
		Root: platform.Sequence[OnboardingState]("onboarding.root",
			newWelcomeStep(deps),
			newObjectiveStep(deps),
			newBudgetStep(deps),
			newCardsStep(deps),
			newCategoriesStep(deps),
			newValuesStep(deps),
			newSummaryStep(deps),
			newConclusionStep(deps),
		),
	}
}

func (d onboardingDeps) suspend(ctx context.Context, s OnboardingState, phase valueobjects.OnboardingPhase, awaiting OnboardingAwaiting, prompt string) (platform.StepOutput[OnboardingState], error) {
	if err := d.Set(ctx, s.UserID, phase.String()); err != nil {
		return platform.StepOutput[OnboardingState]{}, err
	}
	s.Phase = phase
	s.Awaiting = awaiting
	s.Inbound = ""
	s.SuspendedAt = time.Now().UTC()
	return platform.StepOutput[OnboardingState]{
		State:   s,
		Status:  platform.StepStatusSuspended,
		Suspend: &platform.Suspension{Reason: platform.SuspendAwaitingInput, Prompt: prompt},
	}, nil
}

func (d onboardingDeps) advance(ctx context.Context, s OnboardingState, phase valueobjects.OnboardingPhase) (platform.StepOutput[OnboardingState], error) {
	if err := d.Set(ctx, s.UserID, phase.String()); err != nil {
		return platform.StepOutput[OnboardingState]{}, err
	}
	s.Phase = phase
	s.Awaiting = AwaitingNone
	s.Inbound = ""
	s.MessageID = ""
	s.RepromptCount = 0
	return platform.StepOutput[OnboardingState]{State: s, Status: platform.StepStatusCompleted}, nil
}

func fail(err error) (platform.StepOutput[OnboardingState], error) {
	return platform.StepOutput[OnboardingState]{}, err
}

func (d onboardingDeps) record(ctx context.Context, step string, outcome string) {
	d.stepTotal.Add(ctx, 1,
		observability.String("step", step),
		observability.String("outcome", outcome),
	)
}

var categoryOrder = []string{
	"fixed_cost",
	"knowledge",
	"pleasures",
	"goals",
	"financial_freedom",
}

func pendingCategory(values map[string]int64) string {
	if values == nil {
		return categoryOrder[0]
	}
	for _, slug := range categoryOrder {
		if _, ok := values[slug]; !ok {
			return slug
		}
	}
	return ""
}

func copyValues(values map[string]int64) map[string]int64 {
	if values == nil {
		return make(map[string]int64)
	}
	copy := make(map[string]int64, len(values))
	for k, v := range values {
		copy[k] = v
	}
	return copy
}
