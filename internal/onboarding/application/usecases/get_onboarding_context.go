package usecases

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type GetOnboardingContextInput struct {
	UserID uuid.UUID
}

type OnboardingCardView struct {
	Name       string
	ClosingDay int
}

type OnboardingAllocationView struct {
	Kind        string
	BasisPoints int
}

type GetOnboardingContextResult struct {
	Found           bool
	State           valueobjects.OnboardingState
	IncomeCents     int64
	Objective       string
	Cards           []OnboardingCardView
	CustomSplit     []OnboardingAllocationView
	FirstTxRecorded bool
	Phase           string
	WelcomeSent     bool
}

type GetOnboardingContext struct {
	repo appinterfaces.OnboardingSessionRepository
	o11y observability.Observability
}

func NewGetOnboardingContext(repo appinterfaces.OnboardingSessionRepository, o11y observability.Observability) *GetOnboardingContext {
	return &GetOnboardingContext{repo: repo, o11y: o11y}
}

func (uc *GetOnboardingContext) Execute(ctx context.Context, in GetOnboardingContextInput) (GetOnboardingContextResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "onboarding.usecase.get_context")
	defer span.End()

	if in.UserID == uuid.Nil {
		return GetOnboardingContextResult{}, fmt.Errorf("onboarding: get context: user id required")
	}

	session, err := uc.repo.Find(ctx, in.UserID)
	if err != nil {
		if errors.Is(err, appinterfaces.ErrOnboardingSessionNotFound) {
			return GetOnboardingContextResult{Found: false}, nil
		}
		return GetOnboardingContextResult{}, fmt.Errorf("onboarding: get context: find session: %w", err)
	}

	payload := session.Payload()
	cards := make([]OnboardingCardView, 0, len(payload.Cards))
	for _, c := range payload.Cards {
		cards = append(cards, OnboardingCardView{Name: c.Name, ClosingDay: c.ClosingDay})
	}
	splits := make([]OnboardingAllocationView, 0, len(payload.CustomSplit))
	for _, a := range payload.CustomSplit {
		splits = append(splits, OnboardingAllocationView{Kind: a.Kind, BasisPoints: a.BasisPoints})
	}
	return GetOnboardingContextResult{
		Found:           true,
		State:           session.State(),
		IncomeCents:     payload.IncomeCents,
		Objective:       payload.Objective,
		Cards:           cards,
		CustomSplit:     splits,
		FirstTxRecorded: payload.FirstTxRecorded,
		Phase:           payload.Phase,
		WelcomeSent:     payload.WelcomeSentAt != nil,
	}, nil
}
