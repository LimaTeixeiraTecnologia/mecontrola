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

type updateCardUseCase interface {
	Execute(ctx context.Context, in cardinput.UpdateCard) (cardoutput.Card, error)
}

type CardUpdaterAdapter struct {
	cardLister cardListUseCase
	updateUC   updateCardUseCase
}

func NewCardUpdaterAdapter(cardLister cardListUseCase, updateUC updateCardUseCase) *CardUpdaterAdapter {
	return &CardUpdaterAdapter{cardLister: cardLister, updateUC: updateUC}
}

func (a *CardUpdaterAdapter) Execute(ctx context.Context, userID uuid.UUID, in intent.Intent) (appservices.CardUpdaterResult, error) {
	ctx = withWhatsAppPrincipal(ctx, userID)
	cards, err := a.cardLister.Execute(ctx, cardinput.ListCards{UserID: userID, Limit: defaultListCardsLimit})
	if err != nil {
		return appservices.CardUpdaterResult{}, fmt.Errorf("agent: card updater: listar cartões: %w", err)
	}
	resolved, err := resolveCardExact(cards, in.CardName())
	if err != nil {
		return appservices.CardUpdaterResult{}, err
	}
	cardID, err := uuid.Parse(resolved.ID)
	if err != nil {
		return appservices.CardUpdaterResult{}, fmt.Errorf("agent: card updater: card id: %w", err)
	}
	updated, err := a.updateUC.Execute(ctx, cardinput.UpdateCard{
		ID:         cardID,
		UserID:     userID,
		Name:       in.NamePtr(),
		Nickname:   in.NicknamePtr(),
		ClosingDay: in.ClosingDayPtr(),
		DueDay:     in.DueDayPtr(),
	})
	if err != nil {
		return appservices.CardUpdaterResult{}, fmt.Errorf("agent: card updater: atualizar: %w", err)
	}
	return appservices.CardUpdaterResult{
		Nickname:   updated.Nickname,
		Name:       updated.Name,
		ClosingDay: updated.ClosingDay,
		DueDay:     updated.DueDay,
		LimitCents: updated.LimitCents,
	}, nil
}

type softDeleteCardUseCase interface {
	Execute(ctx context.Context, in cardinput.SoftDeleteCard) error
}

type CardDeleterAdapter struct {
	cardLister cardListUseCase
	deleteUC   softDeleteCardUseCase
}

func NewCardDeleterAdapter(cardLister cardListUseCase, deleteUC softDeleteCardUseCase) *CardDeleterAdapter {
	return &CardDeleterAdapter{cardLister: cardLister, deleteUC: deleteUC}
}

func (a *CardDeleterAdapter) Execute(ctx context.Context, userID uuid.UUID, cardName string) (appservices.CardDeleterResult, error) {
	ctx = withWhatsAppPrincipal(ctx, userID)
	cards, err := a.cardLister.Execute(ctx, cardinput.ListCards{UserID: userID, Limit: defaultListCardsLimit})
	if err != nil {
		return appservices.CardDeleterResult{}, fmt.Errorf("agent: card deleter: listar cartões: %w", err)
	}
	resolved, err := resolveCardExact(cards, cardName)
	if err != nil {
		return appservices.CardDeleterResult{}, err
	}
	cardID, err := uuid.Parse(resolved.ID)
	if err != nil {
		return appservices.CardDeleterResult{}, fmt.Errorf("agent: card deleter: card id: %w", err)
	}
	if err := a.deleteUC.Execute(ctx, cardinput.SoftDeleteCard{ID: cardID, UserID: userID}); err != nil {
		return appservices.CardDeleterResult{}, fmt.Errorf("agent: card deleter: apagar: %w", err)
	}
	label := strings.TrimSpace(resolved.Nickname)
	if label == "" {
		label = strings.TrimSpace(resolved.Name)
	}
	return appservices.CardDeleterResult{Name: label}, nil
}

func resolveCardExact(list cardoutput.CardList, name string) (cardoutput.Card, error) {
	target := strings.ToLower(strings.TrimSpace(name))
	if target == "" {
		return cardoutput.Card{}, appservices.ErrAgentCardNotFound
	}
	for _, item := range list.Items {
		if strings.EqualFold(strings.TrimSpace(item.Name), target) {
			return item, nil
		}
		if strings.EqualFold(strings.TrimSpace(item.Nickname), target) {
			return item, nil
		}
	}
	matches := make([]cardoutput.Card, 0, 1)
	for _, item := range list.Items {
		if strings.Contains(strings.ToLower(item.Name), target) || strings.Contains(strings.ToLower(item.Nickname), target) {
			matches = append(matches, item)
		}
	}
	switch len(matches) {
	case 0:
		return cardoutput.Card{}, appservices.ErrAgentCardNotFound
	case 1:
		return matches[0], nil
	default:
		return cardoutput.Card{}, appservices.ErrAgentCardAmbiguous
	}
}
