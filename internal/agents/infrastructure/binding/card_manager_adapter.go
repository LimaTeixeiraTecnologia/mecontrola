package binding

import (
	"context"
	"errors"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	agentsifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	cardinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	cardusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/usecases"
	carddomain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
	txusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases"
)

type cardManagerAdapter struct {
	createCard          *cardusecases.CreateCard
	listCards           *cardusecases.ListCards
	softDeleteCard      *cardusecases.SoftDeleteCard
	hasOpenInstallments *txusecases.HasOpenInstallments
	o11y                observability.Observability
}

func NewCardManagerAdapter(
	createCard *cardusecases.CreateCard,
	listCards *cardusecases.ListCards,
	softDeleteCard *cardusecases.SoftDeleteCard,
	hasOpenInstallments *txusecases.HasOpenInstallments,
	o11y observability.Observability,
) agentsifaces.CardManager {
	return &cardManagerAdapter{
		createCard:          createCard,
		listCards:           listCards,
		softDeleteCard:      softDeleteCard,
		hasOpenInstallments: hasOpenInstallments,
		o11y:                o11y,
	}
}

func (a *cardManagerAdapter) CreateCard(ctx context.Context, in agentsifaces.NewCard) (agentsifaces.CardRef, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.card_manager.create_card")
	defer span.End()

	out, err := a.createCard.Execute(ctx, cardinput.CreateCard{
		UserID:     in.UserID,
		Name:       in.Nickname,
		Nickname:   in.Nickname,
		ClosingDay: in.DueDay,
		LimitCents: 0,
	})
	if err != nil {
		if errors.Is(err, carddomain.ErrNicknameConflict) {
			existing, listErr := a.ListCards(ctx, in.UserID)
			if listErr == nil {
				for _, c := range existing {
					if c.Nickname == in.Nickname {
						return agentsifaces.CardRef{ID: c.ID, Nickname: c.Nickname}, nil
					}
				}
			}
		}
		span.RecordError(err)
		return agentsifaces.CardRef{}, fmt.Errorf("agents/binding/card_manager: criar cartão: %w", err)
	}
	return agentsifaces.CardRef{
		ID:       out.ID,
		Nickname: out.Nickname,
	}, nil
}

func (a *cardManagerAdapter) ListCards(ctx context.Context, userID uuid.UUID) ([]agentsifaces.Card, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.card_manager.list_cards")
	defer span.End()

	out, err := a.listCards.Execute(ctx, cardinput.ListCards{
		UserID: userID,
		Limit:  100,
	})
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("agents/binding/card_manager: listar cartões: %w", err)
	}

	cards := make([]agentsifaces.Card, 0, len(out.Items))
	for _, c := range out.Items {
		cards = append(cards, agentsifaces.Card{
			ID:         c.ID,
			Nickname:   c.Nickname,
			ClosingDay: c.ClosingDay,
			DueDay:     c.DueDay,
			LimitCents: c.LimitCents,
		})
	}
	return cards, nil
}

func (a *cardManagerAdapter) SoftDeleteCard(ctx context.Context, cardID, userID uuid.UUID) error {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.card_manager.soft_delete_card")
	defer span.End()

	if err := a.softDeleteCard.Execute(ctx, cardinput.SoftDeleteCard{
		ID:     cardID,
		UserID: userID,
	}); err != nil {
		span.RecordError(err)
		return fmt.Errorf("agents/binding/card_manager: deletar cartão: %w", err)
	}
	return nil
}

func (a *cardManagerAdapter) HasOpenInstallments(ctx context.Context, cardID, userID uuid.UUID) (bool, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.card_manager.has_open_installments")
	defer span.End()

	exists, err := a.hasOpenInstallments.Execute(ctx, cardID, userID)
	if err != nil {
		span.RecordError(err)
		return false, fmt.Errorf("agents/binding/card_manager: verificar parcelas abertas: %w", err)
	}
	return exists, nil
}
