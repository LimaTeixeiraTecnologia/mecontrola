package dispatcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	cardinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	cardoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
)

var ErrCardsCreateInvalidPayload = errors.New("agent.llm.dispatcher.cards.create: payload invalido")

type ListCardsUseCase interface {
	Execute(ctx context.Context, in cardinput.ListCards) (cardoutput.CardList, error)
}

type GetCardUseCase interface {
	Execute(ctx context.Context, in cardinput.GetCard) (cardoutput.Card, error)
}

type CreateCardUseCase interface {
	Execute(ctx context.Context, in cardinput.CreateCard) (cardoutput.Card, error)
}

type UpdateCardUseCase interface {
	Execute(ctx context.Context, in cardinput.UpdateCard) (cardoutput.Card, error)
}

type UpdateCardLimitUseCase interface {
	Execute(ctx context.Context, in cardinput.UpdateCardLimit) (cardoutput.Card, error)
}

type SoftDeleteCardUseCase interface {
	Execute(ctx context.Context, in cardinput.SoftDeleteCard) error
}

type CardsAdapter struct {
	listUseCase        ListCardsUseCase
	getUseCase         GetCardUseCase
	createUseCase      CreateCardUseCase
	updateUseCase      UpdateCardUseCase
	updateLimitUseCase UpdateCardLimitUseCase
	deleteUseCase      SoftDeleteCardUseCase
	defaultLimit       int
}

func NewCardsAdapter(listUseCase ListCardsUseCase) *CardsAdapter {
	return &CardsAdapter{listUseCase: listUseCase, defaultLimit: 10}
}

func NewCardsAdapterFull(
	listUseCase ListCardsUseCase,
	getUseCase GetCardUseCase,
	createUseCase CreateCardUseCase,
	updateUseCase UpdateCardUseCase,
	updateLimitUseCase UpdateCardLimitUseCase,
	deleteUseCase SoftDeleteCardUseCase,
) *CardsAdapter {
	return &CardsAdapter{
		listUseCase:        listUseCase,
		getUseCase:         getUseCase,
		createUseCase:      createUseCase,
		updateUseCase:      updateUseCase,
		updateLimitUseCase: updateLimitUseCase,
		deleteUseCase:      deleteUseCase,
		defaultLimit:       10,
	}
}

func (a *CardsAdapter) List(ctx context.Context, userID uuid.UUID, _ json.RawMessage) (string, error) {
	out, err := a.listUseCase.Execute(ctx, cardinput.ListCards{
		UserID: userID,
		Limit:  a.defaultLimit,
	})
	if err != nil {
		return "", fmt.Errorf("cards.list: %w", err)
	}
	if len(out.Items) == 0 {
		return "Voce ainda nao tem cartoes cadastrados. Quer cadastrar agora?", nil
	}

	names := make([]string, 0, len(out.Items))
	for _, c := range out.Items {
		label := c.Nickname
		if label == "" {
			label = c.Name
		}
		if label == "" {
			label = "cartao sem nome"
		}
		names = append(names, label)
	}
	return fmt.Sprintf("Voce tem %d cartao(oes): %s.", len(out.Items), strings.Join(names, ", ")), nil
}

type cardsCreatePayload struct {
	Nickname   string `json:"nickname"`
	Name       string `json:"name"`
	ClosingDay int    `json:"closing_day"`
	DueDay     int    `json:"due_day"`
	LimitCents int64  `json:"limit_cents"`
}

func (a *CardsAdapter) Create(ctx context.Context, userID uuid.UUID, rawPayload json.RawMessage) (string, error) {
	if a.createUseCase == nil {
		return "", fmt.Errorf("agent.llm.dispatcher.cards.create: %w", ErrIntentUnsupported)
	}
	if len(rawPayload) == 0 {
		return "", ErrCardsCreateInvalidPayload
	}
	var p cardsCreatePayload
	if err := json.Unmarshal(rawPayload, &p); err != nil {
		return "", fmt.Errorf("agent.llm.dispatcher.cards.create: %w", ErrCardsCreateInvalidPayload)
	}

	nickname := strings.TrimSpace(p.Nickname)
	if nickname == "" {
		return "", fmt.Errorf("agent.llm.dispatcher.cards.create: nickname ausente: %w", ErrCardsCreateInvalidPayload)
	}
	name := strings.TrimSpace(p.Name)
	if name == "" {
		name = nickname
	}
	if p.ClosingDay < 1 || p.ClosingDay > 31 {
		return "", fmt.Errorf("agent.llm.dispatcher.cards.create: closing_day invalido (%d): %w", p.ClosingDay, ErrCardsCreateInvalidPayload)
	}
	if p.DueDay < 1 || p.DueDay > 31 {
		return "", fmt.Errorf("agent.llm.dispatcher.cards.create: due_day invalido (%d): %w", p.DueDay, ErrCardsCreateInvalidPayload)
	}

	created, err := a.createUseCase.Execute(ctx, cardinput.CreateCard{
		UserID:     userID,
		Name:       name,
		Nickname:   nickname,
		ClosingDay: p.ClosingDay,
		DueDay:     p.DueDay,
		LimitCents: p.LimitCents,
	})
	if err != nil {
		return "", fmt.Errorf("cards.create: %w", err)
	}
	label := created.Nickname
	if label == "" {
		label = created.Name
	}
	return fmt.Sprintf("Cartao cadastrado: %s (fecha dia %d, vence dia %d).",
		label, created.ClosingDay, created.DueDay), nil
}

type cardsGetFilters struct {
	ID string `json:"id"`
}

func (a *CardsAdapter) Get(ctx context.Context, userID uuid.UUID, rawFilters json.RawMessage) (string, error) {
	if a.getUseCase == nil {
		return "", fmt.Errorf("cards.get: %w", ErrIntentUnsupported)
	}
	var filters cardsGetFilters
	if err := json.Unmarshal(rawFilters, &filters); err != nil || strings.TrimSpace(filters.ID) == "" {
		return "", fmt.Errorf("cards.get: id ausente")
	}
	cardID, err := uuid.Parse(strings.TrimSpace(filters.ID))
	if err != nil {
		return "", fmt.Errorf("cards.get: id invalido: %w", err)
	}
	out, err := a.getUseCase.Execute(ctx, cardinput.GetCard{ID: cardID, UserID: userID})
	if err != nil {
		return "", fmt.Errorf("cards.get: %w", err)
	}
	label := out.Nickname
	if label == "" {
		label = out.Name
	}
	return fmt.Sprintf("%s: limite R$ %s, fecha dia %d e vence dia %d.",
		label, formatCents(out.LimitCents), out.ClosingDay, out.DueDay,
	), nil
}

type cardsUpdatePayload struct {
	ID          string  `json:"id"`
	Name        *string `json:"name"`
	Nickname    *string `json:"nickname"`
	ClosingDay  *int    `json:"closing_day"`
	DueDay      *int    `json:"due_day"`
	LimitCents  *int64  `json:"limit_cents"`
	VersionHint *int64  `json:"version"`
}

func (a *CardsAdapter) Update(ctx context.Context, userID uuid.UUID, rawPayload json.RawMessage) (string, error) {
	var payload cardsUpdatePayload
	if err := json.Unmarshal(rawPayload, &payload); err != nil || strings.TrimSpace(payload.ID) == "" {
		return "", fmt.Errorf("cards.update: payload invalido")
	}
	cardID, err := uuid.Parse(strings.TrimSpace(payload.ID))
	if err != nil {
		return "", fmt.Errorf("cards.update: id invalido: %w", err)
	}
	if payload.LimitCents != nil {
		if a.updateLimitUseCase == nil {
			return "", fmt.Errorf("cards.update: %w", ErrIntentUnsupported)
		}
		out, err := a.updateLimitUseCase.Execute(ctx, cardinput.UpdateCardLimit{
			CardID:          cardID,
			UserID:          userID,
			LimitCents:      *payload.LimitCents,
			ExpectedVersion: payload.VersionHint,
		})
		if err != nil {
			return "", fmt.Errorf("cards.update_limit: %w", err)
		}
		label := out.Nickname
		if label == "" {
			label = out.Name
		}
		return fmt.Sprintf("Limite do cartao %s atualizado para R$ %s.", label, formatCents(out.LimitCents)), nil
	}
	if a.updateUseCase == nil {
		return "", fmt.Errorf("cards.update: %w", ErrIntentUnsupported)
	}
	out, err := a.updateUseCase.Execute(ctx, cardinput.UpdateCard{
		ID:         cardID,
		UserID:     userID,
		Name:       payload.Name,
		Nickname:   payload.Nickname,
		ClosingDay: payload.ClosingDay,
		DueDay:     payload.DueDay,
	})
	if err != nil {
		return "", fmt.Errorf("cards.update: %w", err)
	}
	label := out.Nickname
	if label == "" {
		label = out.Name
	}
	return fmt.Sprintf("Cartao atualizado: %s (fecha dia %d, vence dia %d).", label, out.ClosingDay, out.DueDay), nil
}

type cardsDeletePayload struct {
	ID string `json:"id"`
}

func (a *CardsAdapter) Delete(ctx context.Context, userID uuid.UUID, rawPayload json.RawMessage) (string, error) {
	if a.deleteUseCase == nil {
		return "", fmt.Errorf("cards.delete: %w", ErrIntentUnsupported)
	}
	var payload cardsDeletePayload
	if err := json.Unmarshal(rawPayload, &payload); err != nil || strings.TrimSpace(payload.ID) == "" {
		return "", fmt.Errorf("cards.delete: payload invalido")
	}
	cardID, err := uuid.Parse(strings.TrimSpace(payload.ID))
	if err != nil {
		return "", fmt.Errorf("cards.delete: id invalido: %w", err)
	}
	if err := a.deleteUseCase.Execute(ctx, cardinput.SoftDeleteCard{ID: cardID, UserID: userID}); err != nil {
		return "", fmt.Errorf("cards.delete: %w", err)
	}
	return "Cartao removido com sucesso.", nil
}
