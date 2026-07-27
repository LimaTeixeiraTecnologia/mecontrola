package tools

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
)

type EditEntryInput struct {
	EntryID           string `json:"entryId,omitempty"`
	AmountCents       int64  `json:"amountCents,omitempty"`
	Description       string `json:"description,omitempty"`
	OccurredAt        string `json:"occurredAt,omitempty"`
	CategoryID        string `json:"categoryId,omitempty"`
	SubcategoryID     string `json:"subcategoryId,omitempty"`
	CategoryVersion   int64  `json:"categoryVersion,omitempty"`
	PaymentMethod     string `json:"paymentMethod,omitempty"`
	SearchAmountCents int64  `json:"searchAmountCents,omitempty"`
	SearchTerm        string `json:"searchTerm,omitempty"`
}

type EditEntryOutput struct {
	NeedsConfirmation bool   `json:"needsConfirmation"`
	ImpactNote        string `json:"impactNote"`
	TargetRef         string `json:"targetRef"`
	Outcome           string `json:"outcome"`
}

func BuildEditEntryTool(editor entryEditor) tool.ToolHandle {
	in := llm.Schema{
		Name:   "edit_entry_input",
		Strict: false,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"entryId":           map[string]any{"type": "string", "description": "Id do lançamento a editar, quando já conhecido (ex.: retornado por search_transactions). Se ausente, searchAmountCents/searchTerm localizam o lançamento."},
				"amountCents":       map[string]any{"type": "integer", "description": "Valor NOVO que o lançamento deve passar a ter. Preencha apenas se o usuário quiser mudar o valor; omita para manter o valor atual."},
				"description":       map[string]any{"type": "string", "description": "Descrição NOVA que o lançamento deve passar a ter. Preencha apenas se o usuário quiser mudar a descrição; omita para manter a atual."},
				"occurredAt":        map[string]any{"type": "string"},
				"categoryId":        map[string]any{"type": "string"},
				"subcategoryId":     map[string]any{"type": "string"},
				"categoryVersion":   map[string]any{"type": "integer"},
				"paymentMethod":     map[string]any{"type": "string", "enum": []string{"pix", "debit_card", "debit_in_account", "cash", "boleto", "ted", "credit_card", "doc", "vale_refeicao", "vale_alimentacao", "transferencia", "apple_pay", "google_pay", "picpay", "mercado_pago", "cheque"}},
				"searchAmountCents": map[string]any{"type": "integer", "description": "Valor ATUAL do lançamento, usado só como critério de busca quando entryId é desconhecido. NUNCA o valor novo pretendido."},
				"searchTerm":        map[string]any{"type": "string", "description": "Termo (descrição atual ou apelido do item, ex.: 'cinema', 'TV') usado só como critério de busca quando entryId é desconhecido."},
			},
			"required":             []string{},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "edit_entry_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"needsConfirmation": map[string]any{"type": "boolean"},
				"impactNote":        map[string]any{"type": "string"},
				"targetRef":         map[string]any{"type": "string"},
				"outcome":           map[string]any{"type": "string"},
			},
			"required":             []string{"needsConfirmation", "impactNote", "targetRef", "outcome"},
			"additionalProperties": false,
		},
	}
	return tool.NewVerbatimTool("edit_entry", "Solicita a edição de um lançamento financeiro (despesa ou receita); aceita valor, descrição, categoria/subcategoria, forma de pagamento e data. Quando entryId não é conhecido, busca lançamentos compatíveis do mês por valor e/ou descrição. A persistência só ocorre após confirmação explícita do usuário.", in, out, buildEditEntryExec(editor), extractEditEntryVerbatim)
}

func extractEditEntryVerbatim(o EditEntryOutput) (string, bool) {
	return o.ImpactNote, o.NeedsConfirmation && o.ImpactNote != ""
}

func buildEditEntryExec(editor entryEditor) func(context.Context, EditEntryInput) (EditEntryOutput, error) {
	return func(ctx context.Context, in EditEntryInput) (EditEntryOutput, error) {
		resourceID, threadID, wamid, itemSeq, ok := agent.InboundExecutionFromContext(ctx)
		if !ok {
			return EditEntryOutput{}, fmt.Errorf("agents.tool.edit_entry: identidade não disponível no contexto")
		}

		userID, err := uuid.Parse(resourceID)
		if err != nil {
			return EditEntryOutput{}, fmt.Errorf("agents.tool.edit_entry: parse resource uuid: %w", err)
		}

		cmd := usecases.EditEntryCommand{
			UserID:   userID,
			ThreadID: threadID,
			WAMID:    wamid,
			ItemSeq:  itemSeq,
		}

		targetID, hasTarget := uuid.UUID{}, false
		if in.EntryID != "" {
			if parsed, parseErr := uuid.Parse(in.EntryID); parseErr == nil {
				targetID, hasTarget = parsed, true
			}
		}

		cmd.AmountCents = in.AmountCents
		cmd.Description = in.Description
		cmd.OccurredAt = in.OccurredAt
		cmd.PaymentMethod = in.PaymentMethod

		if hasTarget {
			cmd.TargetTransactionID = targetID
			if in.CategoryID != "" {
				catID, catErr := uuid.Parse(in.CategoryID)
				if catErr != nil {
					return EditEntryOutput{}, fmt.Errorf("agents.tool.edit_entry: parse categoryId: %w", catErr)
				}
				cmd.CategoryID = catID
			}
			if in.SubcategoryID != "" {
				subID, subErr := uuid.Parse(in.SubcategoryID)
				if subErr != nil {
					return EditEntryOutput{}, fmt.Errorf("agents.tool.edit_entry: parse subcategoryId: %w", subErr)
				}
				cmd.SubcategoryID = subID
			}
			cmd.CategoryVersion = in.CategoryVersion
		} else {
			cmd.SearchAmountCents = in.SearchAmountCents
			cmd.SearchTerm = in.SearchTerm
		}

		result, err := editor.EditEntry(ctx, cmd)
		if err != nil {
			return EditEntryOutput{}, fmt.Errorf("agents.tool.edit_entry: %w", err)
		}

		impact := result.Message
		if impact == "" {
			impact = "Este lançamento será atualizado com os novos dados. Por favor confirme."
		}
		return EditEntryOutput{
			NeedsConfirmation: true,
			ImpactNote:        impact,
			TargetRef:         in.EntryID,
			Outcome:           result.Outcome.String(),
		}, nil
	}
}
