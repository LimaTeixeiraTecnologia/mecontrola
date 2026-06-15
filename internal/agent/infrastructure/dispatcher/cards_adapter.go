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

type CreateCardUseCase interface {
	Execute(ctx context.Context, in cardinput.CreateCard) (cardoutput.Card, error)
}

type CardsAdapter struct {
	listUseCase   ListCardsUseCase
	createUseCase CreateCardUseCase
	defaultLimit  int
}

func NewCardsAdapter(listUseCase ListCardsUseCase) *CardsAdapter {
	return &CardsAdapter{listUseCase: listUseCase, defaultLimit: 10}
}

func NewCardsAdapterFull(listUseCase ListCardsUseCase, createUseCase CreateCardUseCase) *CardsAdapter {
	return &CardsAdapter{listUseCase: listUseCase, createUseCase: createUseCase, defaultLimit: 10}
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
