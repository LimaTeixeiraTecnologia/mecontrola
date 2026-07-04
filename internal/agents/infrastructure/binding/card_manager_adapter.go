package binding

import (
	"context"
	"errors"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	agentsifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	cardinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	cardoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	cardusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/usecases"
	carddomain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
	txusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases"
)

type cardManagerAdapter struct {
	createCard          *cardusecases.CreateCard
	listCards           *cardusecases.ListCards
	getCard             *cardusecases.GetCard
	resolveByNickname   *cardusecases.ResolveCardByNickname
	countCards          *cardusecases.CountCards
	bestPurchaseDay     *cardusecases.BestPurchaseDay
	updateCard          *cardusecases.UpdateCard
	softDeleteCard      *cardusecases.SoftDeleteCard
	hasOpenInstallments *txusecases.HasOpenInstallments
	o11y                observability.Observability
}

func NewCardManagerAdapter(
	createCard *cardusecases.CreateCard,
	listCards *cardusecases.ListCards,
	getCard *cardusecases.GetCard,
	resolveByNickname *cardusecases.ResolveCardByNickname,
	countCards *cardusecases.CountCards,
	bestPurchaseDay *cardusecases.BestPurchaseDay,
	updateCard *cardusecases.UpdateCard,
	softDeleteCard *cardusecases.SoftDeleteCard,
	hasOpenInstallments *txusecases.HasOpenInstallments,
	o11y observability.Observability,
) agentsifaces.CardManager {
	return &cardManagerAdapter{
		createCard:          createCard,
		listCards:           listCards,
		getCard:             getCard,
		resolveByNickname:   resolveByNickname,
		countCards:          countCards,
		bestPurchaseDay:     bestPurchaseDay,
		updateCard:          updateCard,
		softDeleteCard:      softDeleteCard,
		hasOpenInstallments: hasOpenInstallments,
		o11y:                o11y,
	}
}

func (a *cardManagerAdapter) CreateCard(ctx context.Context, in agentsifaces.NewCard) (agentsifaces.CardRef, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.card_manager.create_card")
	defer span.End()

	out, err := a.createCard.Execute(ctx, cardinput.CreateCard{
		UserID:   in.UserID,
		Nickname: in.Nickname,
		Bank:     in.Bank,
		DueDay:   in.DueDay,
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
			ID:              c.ID,
			Nickname:        c.Nickname,
			Bank:            c.Bank,
			ClosingDay:      c.ClosingDay,
			DueDay:          c.DueDay,
			BestPurchaseDay: c.BestPurchaseDay,
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

func (a *cardManagerAdapter) GetCard(ctx context.Context, cardID, userID uuid.UUID) (agentsifaces.Card, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.card_manager.get_card")
	defer span.End()

	out, err := a.getCard.Execute(ctx, cardinput.GetCard{ID: cardID, UserID: userID})
	if err != nil {
		span.RecordError(err)
		return agentsifaces.Card{}, fmt.Errorf("agents/binding/card_manager: obter cartão: %w", err)
	}
	return mapCardOutput(out), nil
}

func (a *cardManagerAdapter) ResolveCardByNickname(ctx context.Context, userID uuid.UUID, nickname string) (agentsifaces.Card, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.card_manager.resolve_card_by_nickname")
	defer span.End()

	out, err := a.resolveByNickname.Execute(ctx, cardinput.ResolveCardByNickname{UserID: userID, Nickname: nickname})
	if err != nil {
		if errors.Is(err, carddomain.ErrCardNotFound) {
			return agentsifaces.Card{}, agentsifaces.ErrCardNotFound
		}
		span.RecordError(err)
		return agentsifaces.Card{}, fmt.Errorf("agents/binding/card_manager: resolver cartão por apelido: %w", err)
	}
	return mapCardOutput(out), nil
}

func (a *cardManagerAdapter) CountCards(ctx context.Context, userID uuid.UUID) (int64, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.card_manager.count_cards")
	defer span.End()

	out, err := a.countCards.Execute(ctx, cardinput.CountCards{UserID: userID})
	if err != nil {
		span.RecordError(err)
		return 0, fmt.Errorf("agents/binding/card_manager: contar cartões: %w", err)
	}
	return out.Total, nil
}

func (a *cardManagerAdapter) BestPurchaseDay(ctx context.Context, bank string, dueDay int) (agentsifaces.BestPurchaseDay, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.card_manager.best_purchase_day")
	defer span.End()

	out, err := a.bestPurchaseDay.Execute(ctx, cardinput.BestPurchaseDay{Bank: bank, DueDay: dueDay})
	if err != nil {
		span.RecordError(err)
		return agentsifaces.BestPurchaseDay{}, fmt.Errorf("agents/binding/card_manager: melhor dia de compra: %w", err)
	}
	return agentsifaces.BestPurchaseDay{
		ClosingDay:      out.ClosingDay,
		BestPurchaseDay: out.BestPurchaseDay,
	}, nil
}

func (a *cardManagerAdapter) UpdateCard(ctx context.Context, in agentsifaces.CardUpdate) (agentsifaces.Card, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.card_manager.update_card")
	defer span.End()

	out, err := a.updateCard.Execute(ctx, cardinput.UpdateCard{
		ID:       in.ID,
		UserID:   in.UserID,
		Nickname: in.Nickname,
		Bank:     in.Bank,
		DueDay:   in.DueDay,
	})
	if err != nil {
		span.RecordError(err)
		return agentsifaces.Card{}, fmt.Errorf("agents/binding/card_manager: atualizar cartão: %w", err)
	}
	return mapCardOutput(out), nil
}

func mapCardOutput(c cardoutput.Card) agentsifaces.Card {
	return agentsifaces.Card{
		ID:              c.ID,
		UserID:          c.UserID,
		Nickname:        c.Nickname,
		Bank:            c.Bank,
		ClosingDay:      c.ClosingDay,
		DueDay:          c.DueDay,
		BestPurchaseDay: c.BestPurchaseDay,
		CreatedAt:       c.CreatedAt,
		UpdatedAt:       c.UpdatedAt,
	}
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
