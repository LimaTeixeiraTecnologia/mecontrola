package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
)

type ResolveCardInput struct {
	Nickname string `json:"nickname"`
}

type ResolveCardOutput struct {
	Found    bool   `json:"found"`
	CardID   string `json:"cardId"`
	Nickname string `json:"nickname"`
	Bank     string `json:"bank"`
	DueDay   int    `json:"dueDay"`
}

func BuildResolveCardTool(cards interfaces.CardManager) tool.ToolHandle {
	in := llm.Schema{
		Name:   "resolve_card_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"nickname": map[string]any{"type": "string"},
			},
			"required":             []string{"nickname"},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "resolve_card_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"found":    map[string]any{"type": "boolean"},
				"cardId":   map[string]any{"type": "string"},
				"nickname": map[string]any{"type": "string"},
				"bank":     map[string]any{"type": "string"},
				"dueDay":   map[string]any{"type": "integer"},
			},
			"required":             []string{"found", "cardId", "nickname", "bank", "dueDay"},
			"additionalProperties": false,
		},
	}
	return tool.NewTool("resolve_card", "Resolve o cartão de crédito do usuário pelo apelido informado, retornando o cardId; use como etapa obrigatória antes de registrar compra no crédito OU antes de consultar a fatura do cartão via query_card_invoice. O nickname é qualquer nome que o usuário use para o cartão (apelido, banco ou marca, ex.: 'nubank'): passe a palavra exata que o usuário citou, sem pedir confirmação do apelido.", in, out, buildResolveCardExec(cards))
}

func buildResolveCardExec(cards interfaces.CardManager) func(context.Context, ResolveCardInput) (ResolveCardOutput, error) {
	return func(ctx context.Context, in ResolveCardInput) (ResolveCardOutput, error) {
		resourceID, _, _, ok := agent.InboundIdentityFromContext(ctx)
		if !ok {
			return ResolveCardOutput{}, fmt.Errorf("resolve_card: identidade não disponível no contexto")
		}
		userID, err := uuid.Parse(resourceID)
		if err != nil {
			return ResolveCardOutput{}, fmt.Errorf("resolve_card: userId inválido: %w", err)
		}
		c, err := cards.ResolveCardByNickname(ctx, userID, in.Nickname)
		if err != nil {
			if errors.Is(err, interfaces.ErrCardNotFound) {
				return ResolveCardOutput{Found: false}, nil
			}
			return ResolveCardOutput{}, fmt.Errorf("resolve_card: %w", err)
		}
		return ResolveCardOutput{
			Found:    true,
			CardID:   c.ID,
			Nickname: c.Nickname,
			Bank:     c.Bank,
			DueDay:   c.DueDay,
		}, nil
	}
}
