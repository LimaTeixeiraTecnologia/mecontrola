package binding

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	cardinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	cardoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
)

type createCardUseCase interface {
	Execute(ctx context.Context, in cardinput.CreateCard) (cardoutput.Card, error)
}

type CardCreatorAdapter struct {
	uc createCardUseCase
}

func NewCardCreatorAdapter(uc createCardUseCase) *CardCreatorAdapter {
	return &CardCreatorAdapter{uc: uc}
}

func (a *CardCreatorAdapter) Execute(ctx context.Context, userID uuid.UUID, in intent.Intent) (appservices.CardCreatorResult, error) {
	name := strings.TrimSpace(in.CardName())
	if name == "" {
		name = strings.TrimSpace(in.CardNickname())
	}
	created, err := a.uc.Execute(ctx, cardinput.CreateCard{
		UserID:     userID,
		Name:       name,
		Nickname:   in.CardNickname(),
		ClosingDay: in.ClosingDay(),
		DueDay:     in.DueDay(),
		LimitCents: in.LimitCents(),
	})
	if err != nil {
		return appservices.CardCreatorResult{}, fmt.Errorf("agent: card creator: %w", err)
	}
	return appservices.CardCreatorResult{
		Nickname:   created.Nickname,
		Name:       created.Name,
		ClosingDay: created.ClosingDay,
		DueDay:     created.DueDay,
		LimitCents: created.LimitCents,
	}, nil
}

type countCardsUseCase interface {
	Execute(ctx context.Context, in cardinput.CountCards) (cardoutput.CardCount, error)
}

type CardCounterAdapter struct {
	uc countCardsUseCase
}

func NewCardCounterAdapter(uc countCardsUseCase) *CardCounterAdapter {
	return &CardCounterAdapter{uc: uc}
}

func (a *CardCounterAdapter) Execute(ctx context.Context, userID uuid.UUID) (int64, error) {
	out, err := a.uc.Execute(ctx, cardinput.CountCards{UserID: userID})
	if err != nil {
		return 0, fmt.Errorf("agent: card counter: %w", err)
	}
	return out.Total, nil
}
