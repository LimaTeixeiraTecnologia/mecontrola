package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
)

type RegisterCardPurchaseInput struct {
	Wamid             string     `json:"wamid"`
	ItemSeq           int        `json:"itemSeq"`
	UserID            string     `json:"userId"`
	CardNickname      string     `json:"cardNickname"`
	TotalAmountCents  int64      `json:"totalAmountCents"`
	InstallmentsTotal int        `json:"installmentsTotal"`
	Description       string     `json:"description"`
	PurchasedAt       string     `json:"purchasedAt,omitempty"`
	CategoryID        *uuid.UUID `json:"categoryId,omitempty"`
	SubcategoryID     *uuid.UUID `json:"subcategoryId,omitempty"`
}

type RegisterCardPurchaseOutput struct {
	ResourceID string `json:"resourceId"`
	Kind       string `json:"kind"`
	IsReplay   bool   `json:"isReplay"`
}

func BuildRegisterCardPurchaseTool(ledger interfaces.TransactionsLedger, cardManager interfaces.CardManager, writer idempotentWriter) tool.ToolHandle {
	in := llm.Schema{
		Name:   "register_card_purchase_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"wamid":             map[string]any{"type": "string"},
				"itemSeq":           map[string]any{"type": "integer"},
				"userId":            map[string]any{"type": "string"},
				"cardNickname":      map[string]any{"type": "string"},
				"totalAmountCents":  map[string]any{"type": "integer"},
				"installmentsTotal": map[string]any{"type": "integer", "minimum": 1, "maximum": 24},
				"description":       map[string]any{"type": "string"},
				"purchasedAt":       map[string]any{"type": "string"},
				"categoryId":        map[string]any{"type": "string"},
				"subcategoryId":     map[string]any{"type": "string"},
			},
			"required":             []string{"wamid", "itemSeq", "userId", "cardNickname", "totalAmountCents", "installmentsTotal", "description"},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "register_card_purchase_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"resourceId": map[string]any{"type": "string"},
				"kind":       map[string]any{"type": "string"},
				"isReplay":   map[string]any{"type": "boolean"},
			},
			"required":             []string{"resourceId", "kind", "isReplay"},
			"additionalProperties": false,
		},
	}
	return tool.NewTool[RegisterCardPurchaseInput, RegisterCardPurchaseOutput]("register_card_purchase", "Registra uma compra no cartão de crédito no ledger financeiro do usuário.", in, out, buildRegisterCardPurchaseExec(ledger, cardManager, writer))
}

func buildRegisterCardPurchaseExec(ledger interfaces.TransactionsLedger, cardManager interfaces.CardManager, writer idempotentWriter) func(context.Context, RegisterCardPurchaseInput) (RegisterCardPurchaseOutput, error) {
	return func(ctx context.Context, in RegisterCardPurchaseInput) (RegisterCardPurchaseOutput, error) {
		if in.InstallmentsTotal < 1 || in.InstallmentsTotal > 24 {
			return RegisterCardPurchaseOutput{}, fmt.Errorf("register_card_purchase: parcelas deve estar entre 1 e 24, recebido %d", in.InstallmentsTotal)
		}
		userID, err := uuid.Parse(in.UserID)
		if err != nil {
			return RegisterCardPurchaseOutput{}, fmt.Errorf("register_card_purchase: userId inválido: %w", err)
		}
		purchasedAt := in.PurchasedAt
		if purchasedAt == "" {
			loc, locErr := time.LoadLocation("America/Sao_Paulo")
			if locErr != nil {
				loc = time.UTC
			}
			purchasedAt = time.Now().In(loc).Format("2006-01-02")
		}
		catID := uuid.Nil
		if in.CategoryID != nil {
			catID = *in.CategoryID
		}
		result, writeErr := writer.Execute(ctx, userID, in.Wamid, in.ItemSeq, "create_card_purchase", "card_purchase", func(ctx context.Context) (uuid.UUID, bool, error) {
			cards, listErr := cardManager.ListCards(ctx, userID)
			if listErr != nil {
				return uuid.Nil, false, fmt.Errorf("listar cartões: %w", listErr)
			}
			var cardID uuid.UUID
			found := false
			for _, c := range cards {
				if strings.EqualFold(c.Nickname, in.CardNickname) {
					parsed, parseErr := uuid.Parse(c.ID)
					if parseErr != nil {
						return uuid.Nil, false, fmt.Errorf("register_card_purchase: id de cartão inválido: %w", parseErr)
					}
					cardID = parsed
					found = true
					break
				}
			}
			if !found {
				return uuid.Nil, false, fmt.Errorf("register_card_purchase: cartão não encontrado: %q", in.CardNickname)
			}
			ref, err := ledger.CreateCardPurchase(ctx, interfaces.RawCardPurchase{
				CardID:            cardID,
				TotalAmountCents:  in.TotalAmountCents,
				InstallmentsTotal: in.InstallmentsTotal,
				Description:       in.Description,
				CategoryID:        catID,
				SubcategoryID:     in.SubcategoryID,
				PurchasedAt:       purchasedAt,
				OriginWamid:       in.Wamid,
				OriginItemSeq:     in.ItemSeq,
				OriginOperation:   "create_card_purchase",
			})
			if err != nil {
				return uuid.Nil, false, err
			}
			return ref.ID, ref.Reconciled, nil
		})
		if writeErr != nil {
			return RegisterCardPurchaseOutput{}, fmt.Errorf("register_card_purchase: %w", writeErr)
		}
		return RegisterCardPurchaseOutput{
			ResourceID: result.ResourceID.String(),
			Kind:       "card_purchase",
			IsReplay:   result.Outcome == agent.ToolOutcomeReplay,
		}, nil
	}
}
