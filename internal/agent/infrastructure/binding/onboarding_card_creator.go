package binding

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	cardinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	cardoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
)

type onboardingCardCreatorUseCase interface {
	Execute(ctx context.Context, in cardinput.CreateCard) (cardoutput.Card, error)
}

type OnboardingCardCreatorAdapter struct {
	uc onboardingCardCreatorUseCase
}

func NewOnboardingCardCreatorAdapter(uc onboardingCardCreatorUseCase) *OnboardingCardCreatorAdapter {
	return &OnboardingCardCreatorAdapter{uc: uc}
}

func (a *OnboardingCardCreatorAdapter) Execute(ctx context.Context, userID, nickname string, dueDay int) error {
	if a.uc == nil {
		return nil
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("agent: onboarding card creator: user id: %w", err)
	}
	closingDay := dueDay - 7
	if closingDay < 1 {
		closingDay += 30
	}
	_, err = a.uc.Execute(ctx, cardinput.CreateCard{
		UserID:     uid,
		Name:       nickname,
		Nickname:   nickname,
		ClosingDay: closingDay,
		DueDay:     dueDay,
		LimitCents: 0,
	})
	if err != nil {
		return fmt.Errorf("agent: onboarding card creator: %w", err)
	}
	return nil
}
