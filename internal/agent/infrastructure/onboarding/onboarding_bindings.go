package onboarding

import (
	"context"

	"github.com/google/uuid"

	agentwf "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow"
	onbinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/input"
	onbusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type welcomeMarkerBinding struct {
	uc *onbusecases.MarkWelcomeSent
}

func NewWelcomeMarkerBinding(uc *onbusecases.MarkWelcomeSent) agentwf.WelcomeMarker {
	if uc == nil {
		return nil
	}
	return &welcomeMarkerBinding{uc: uc}
}

func (b *welcomeMarkerBinding) Mark(ctx context.Context, userID uuid.UUID) (bool, error) {
	out, err := b.uc.Execute(ctx, onbusecases.MarkWelcomeSentInput{UserID: userID})
	if err != nil {
		return false, err
	}
	return out.AlreadySent, nil
}

type objectiveSaverBinding struct {
	uc *onbusecases.SaveOnboardingObjective
}

func NewObjectiveSaverBinding(uc *onbusecases.SaveOnboardingObjective) agentwf.ObjectiveSaver {
	if uc == nil {
		return nil
	}
	return &objectiveSaverBinding{uc: uc}
}

func (b *objectiveSaverBinding) Save(ctx context.Context, userID uuid.UUID, objective string) error {
	_, err := b.uc.Execute(ctx, onbusecases.SaveOnboardingObjectiveInput{
		UserID:    userID,
		Objective: objective,
	})
	return err
}

type incomeSaverBinding struct {
	uc *onbusecases.SaveOnboardingIncome
}

func NewIncomeSaverBinding(uc *onbusecases.SaveOnboardingIncome) agentwf.IncomeSaver {
	if uc == nil {
		return nil
	}
	return &incomeSaverBinding{uc: uc}
}

func (b *incomeSaverBinding) Save(ctx context.Context, userID uuid.UUID, incomeCents int64) error {
	_, err := b.uc.Execute(ctx, onbusecases.SaveOnboardingIncomeInput{
		UserID:      userID,
		IncomeCents: incomeCents,
	})
	return err
}

type cardSaverBinding struct {
	uc *onbusecases.SaveOnboardingCard
}

func NewCardSaverBinding(uc *onbusecases.SaveOnboardingCard) agentwf.CardSaver {
	if uc == nil {
		return nil
	}
	return &cardSaverBinding{uc: uc}
}

func (b *cardSaverBinding) Save(ctx context.Context, userID uuid.UUID, nickname string, dueDay int) error {
	_, err := b.uc.Execute(ctx, onbinput.SaveOnboardingCardInput{
		UserID:   userID,
		Nickname: nickname,
		DueDay:   dueDay,
	})
	return err
}

type splitsSaverBinding struct {
	uc *onbusecases.SaveOnboardingBudgetSplits
}

func NewSplitsSaverBinding(uc *onbusecases.SaveOnboardingBudgetSplits) agentwf.SplitsSaver {
	if uc == nil {
		return nil
	}
	return &splitsSaverBinding{uc: uc}
}

func (b *splitsSaverBinding) Save(ctx context.Context, userID uuid.UUID, values map[string]int64) (bool, error) {
	items := make([]onbusecases.BudgetSplitItem, 0, len(values))
	for slug, amount := range values {
		kind, ok := slugToCategoryKind[slug]
		if !ok {
			continue
		}
		items = append(items, onbusecases.BudgetSplitItem{Kind: kind, AmountCents: amount})
	}
	out, err := b.uc.Execute(ctx, onbusecases.SaveOnboardingBudgetSplitsInput{
		UserID:      userID,
		Allocations: items,
	})
	if err != nil {
		return false, err
	}
	return out.Applied, nil
}

type phaseSetterBinding struct {
	uc *onbusecases.SetOnboardingPhase
}

func NewPhaseSetterBinding(uc *onbusecases.SetOnboardingPhase) agentwf.PhaseSetter {
	if uc == nil {
		return nil
	}
	return &phaseSetterBinding{uc: uc}
}

func (b *phaseSetterBinding) Set(ctx context.Context, userID uuid.UUID, phase string) error {
	p, err := valueobjects.ParseOnboardingPhase(phase)
	if err != nil {
		return err
	}
	_, err = b.uc.Execute(ctx, onbusecases.SetOnboardingPhaseInput{UserID: userID, Phase: p})
	return err
}

type sessionCompleterBinding struct {
	uc *onbusecases.CompleteOnboardingSession
}

func NewSessionCompleterBinding(uc *onbusecases.CompleteOnboardingSession) agentwf.SessionCompleter {
	if uc == nil {
		return nil
	}
	return &sessionCompleterBinding{uc: uc}
}

func (b *sessionCompleterBinding) Complete(ctx context.Context, userID uuid.UUID) error {
	_, err := b.uc.Execute(ctx, onbusecases.CompleteOnboardingSessionInput{UserID: userID})
	return err
}

type contextLoaderBinding struct {
	uc *onbusecases.GetOnboardingContext
}

func NewContextLoaderBinding(uc *onbusecases.GetOnboardingContext) agentwf.ContextLoader {
	if uc == nil {
		return nil
	}
	return &contextLoaderBinding{uc: uc}
}

func (b *contextLoaderBinding) Load(ctx context.Context, userID uuid.UUID) (agentwf.OnboardingContext, error) {
	out, err := b.uc.Execute(ctx, onbusecases.GetOnboardingContextInput{UserID: userID})
	if err != nil {
		return agentwf.OnboardingContext{}, err
	}
	cards := make([]agentwf.OnboardingCardState, 0, len(out.Cards))
	for _, c := range out.Cards {
		cards = append(cards, agentwf.OnboardingCardState{Name: c.Name, DueDay: c.DueDay})
	}
	return agentwf.OnboardingContext{
		Objective:   out.Objective,
		IncomeCents: out.IncomeCents,
		Cards:       cards,
	}, nil
}
