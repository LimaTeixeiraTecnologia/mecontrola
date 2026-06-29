package bootstrap

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	cardinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	cardoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
)

type createCardUseCase interface {
	Execute(ctx context.Context, in cardinput.CreateCard) (cardoutput.Card, error)
}

type OnboardingCardCreatorAdapter struct {
	uc createCardUseCase
}

func NewOnboardingCardCreatorAdapter(uc createCardUseCase) *OnboardingCardCreatorAdapter {
	return &OnboardingCardCreatorAdapter{uc: uc}
}

func (a *OnboardingCardCreatorAdapter) Execute(ctx context.Context, userID, nickname string, closingDay int) error {
	if a.uc == nil {
		return nil
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("bootstrap: onboarding card creator: user id: %w", err)
	}
	_, err = a.uc.Execute(ctx, cardinput.CreateCard{
		UserID:     uid,
		Name:       nickname,
		Nickname:   nickname,
		ClosingDay: closingDay,
		LimitCents: 0,
	})
	if err != nil {
		return fmt.Errorf("bootstrap: onboarding card creator: %w", err)
	}
	return nil
}
