package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

const (
	DestructiveConfirmWorkflowID = "destructive-confirm"
	StepEvaluateResponseID       = "evaluate-response"
	confirmTTL                   = 5 * time.Minute
)

func DestructiveConfirmKey(resourceID string) string {
	return resourceID + ":confirm"
}

func BuildDestructiveConfirmWorkflow(ledger interfaces.TransactionsLedger, cards interfaces.CardManager, categories interfaces.CategoriesReader, recurrences interfaces.RecurrenceManager) workflow.Definition[ConfirmState] {
	step := workflow.NewStepFunc(StepEvaluateResponseID, buildEvalStep(ledger, cards, categories, recurrences))
	return workflow.Definition[ConfirmState]{
		ID:          DestructiveConfirmWorkflowID,
		Root:        step,
		Durable:     true,
		MaxAttempts: 1,
	}
}

type executeFn func(ctx context.Context, state ConfirmState) error

func buildExecMap(ledger interfaces.TransactionsLedger, cards interfaces.CardManager, categories interfaces.CategoriesReader, recurrences interfaces.RecurrenceManager) map[OperationKind]executeFn {
	return map[OperationKind]executeFn{
		OpDeleteEntry: func(ctx context.Context, state ConfirmState) error {
			return executeDeleteEntry(ctx, state, ledger)
		},
		OpEditEntry: func(ctx context.Context, state ConfirmState) error {
			return executeEditEntry(ctx, state, ledger)
		},
		OpDeleteCard: func(ctx context.Context, state ConfirmState) error {
			return executeDeleteCard(ctx, state, cards)
		},
		OpConfirmRegister: func(ctx context.Context, state ConfirmState) error {
			return executeRegister(ctx, state, ledger, categories)
		},
		OpUpdateRecurrence: func(ctx context.Context, state ConfirmState) error {
			return executeUpdateRecurrence(ctx, state, recurrences)
		},
		OpDeleteRecurrence: func(ctx context.Context, state ConfirmState) error {
			return executeDeleteRecurrence(ctx, state, recurrences)
		},
		OpUpdateCard: func(ctx context.Context, state ConfirmState) error {
			return executeUpdateCard(ctx, state, cards)
		},
	}
}

func buildEvalStep(ledger interfaces.TransactionsLedger, cards interfaces.CardManager, categories interfaces.CategoriesReader, recurrences interfaces.RecurrenceManager) func(context.Context, ConfirmState) (workflow.StepOutput[ConfirmState], error) {
	execMap := buildExecMap(ledger, cards, categories, recurrences)
	return func(ctx context.Context, state ConfirmState) (workflow.StepOutput[ConfirmState], error) {
		if state.Operation == OpConfirmRegister {
			return evalRegisterClarification(ctx, state, execMap)
		}

		if state.ResumeText == "" {
			state.ResponseText = buildConfirmQuestion(state)
			return workflow.StepOutput[ConfirmState]{
				State:  state,
				Status: workflow.StepStatusSuspended,
				Suspend: &workflow.Suspension{
					Reason: workflow.SuspendAwaitingInput,
					Prompt: state.ResponseText,
				},
			}, nil
		}

		if time.Since(state.SuspendedAt) > confirmTTL {
			state.ResponseText = "⏱️ O tempo para confirmação expirou. Operação cancelada."
			state.Awaiting = AwaitingNone
			return workflow.StepOutput[ConfirmState]{State: state, Status: workflow.StepStatusCompleted}, nil
		}

		text := strings.ToLower(strings.TrimSpace(state.ResumeText))

		if isSim(text) {
			if err := executeOperation(ctx, state, execMap); err != nil {
				state.ResponseText = fmt.Sprintf("❌ Não foi possível realizar a operação: %s", err.Error())
				return workflow.StepOutput[ConfirmState]{State: state, Status: workflow.StepStatusFailed}, fmt.Errorf("workflows.destructive_confirm: execute: %w", err)
			}
			state.ResponseText = successMessage(state.Operation)
			state.Awaiting = AwaitingNone
			return workflow.StepOutput[ConfirmState]{State: state, Status: workflow.StepStatusCompleted}, nil
		}

		if isNao(text) {
			state.ResponseText = "🚫 Operação cancelada conforme solicitado."
			state.Awaiting = AwaitingNone
			return workflow.StepOutput[ConfirmState]{State: state, Status: workflow.StepStatusCompleted}, nil
		}

		if state.RepromptDone {
			state.ResponseText = "🚫 Operação cancelada: resposta não reconhecida."
			state.Awaiting = AwaitingNone
			return workflow.StepOutput[ConfirmState]{State: state, Status: workflow.StepStatusCompleted}, nil
		}

		state.RepromptDone = true
		state.ResumeText = ""
		state.ResponseText = "Não entendi. Por favor, responda apenas *sim* ou *não* para confirmar a operação."
		return workflow.StepOutput[ConfirmState]{
			State:  state,
			Status: workflow.StepStatusSuspended,
			Suspend: &workflow.Suspension{
				Reason: workflow.SuspendAwaitingInput,
				Prompt: state.ResponseText,
			},
		}, nil
	}
}

func evalRegisterClarification(ctx context.Context, state ConfirmState, execMap map[OperationKind]executeFn) (workflow.StepOutput[ConfirmState], error) {
	if state.ResumeText == "" {
		state.ResponseText = "Qual é a categoria deste lançamento?"
		return workflow.StepOutput[ConfirmState]{
			State:  state,
			Status: workflow.StepStatusSuspended,
			Suspend: &workflow.Suspension{
				Reason: workflow.SuspendAwaitingInput,
				Prompt: state.ResponseText,
			},
		}, nil
	}

	if time.Since(state.SuspendedAt) > confirmTTL {
		state.ResponseText = "⏱️ O tempo para confirmação expirou. Lançamento cancelado."
		state.Awaiting = AwaitingNone
		return workflow.StepOutput[ConfirmState]{State: state, Status: workflow.StepStatusCompleted}, nil
	}

	if err := executeOperation(ctx, state, execMap); err != nil {
		if state.RepromptDone {
			state.ResponseText = "🚫 Não foi possível identificar a categoria. Lançamento cancelado."
			state.Awaiting = AwaitingNone
			return workflow.StepOutput[ConfirmState]{State: state, Status: workflow.StepStatusCompleted}, nil
		}
		state.RepromptDone = true
		state.ResumeText = ""
		state.ResponseText = "Não reconheci essa categoria. Qual é a categoria deste lançamento?"
		return workflow.StepOutput[ConfirmState]{
			State:  state,
			Status: workflow.StepStatusSuspended,
			Suspend: &workflow.Suspension{
				Reason: workflow.SuspendAwaitingInput,
				Prompt: state.ResponseText,
			},
		}, nil
	}

	state.ResponseText = successMessage(state.Operation)
	state.Awaiting = AwaitingNone
	return workflow.StepOutput[ConfirmState]{State: state, Status: workflow.StepStatusCompleted}, nil
}

func ContinueDestructiveConfirm(
	ctx context.Context,
	engine workflow.Engine[ConfirmState],
	def workflow.Definition[ConfirmState],
	key string,
	userMessage string,
) (bool, string, error) {
	resumeBytes, err := json.Marshal(map[string]string{"resumeText": userMessage})
	if err != nil {
		return false, "", fmt.Errorf("workflows.destructive_confirm: marshal resume: %w", err)
	}

	result, resumeErr := engine.Resume(ctx, def, key, resumeBytes)
	if result.Status == 0 && resumeErr == nil {
		return false, "", nil
	}

	if resumeErr != nil {
		return true, result.State.ResponseText, fmt.Errorf("workflows.destructive_confirm: resume: %w", resumeErr)
	}

	return true, result.State.ResponseText, nil
}

func buildConfirmQuestion(state ConfirmState) string {
	base := "⚠️ Você confirma esta operação?\n\n" + state.ImpactNote
	return base + "\n\nResponda *sim* para confirmar ou *não* para cancelar."
}

func successMessage(op OperationKind) string {
	switch op {
	case OpDeleteEntry:
		return "✅ Lançamento excluído com sucesso."
	case OpEditEntry:
		return "✅ Lançamento atualizado com sucesso."
	case OpDeleteCard:
		return "✅ Cartão removido com sucesso."
	case OpConfirmRegister:
		return "✅ Lançamento registrado com sucesso."
	case OpUpdateRecurrence:
		return "✅ Recorrência atualizada com sucesso."
	case OpDeleteRecurrence:
		return "✅ Recorrência removida com sucesso."
	case OpUpdateCard:
		return "✅ Cartão atualizado com sucesso."
	default:
		return "✅ Operação realizada com sucesso."
	}
}

func isSim(s string) bool {
	switch s {
	case "sim", "confirmar", "confirmo", "ok", "pode", "yes", "s":
		return true
	default:
		return false
	}
}

func isNao(s string) bool {
	switch s {
	case "não", "nao", "cancelar", "cancelo", "no", "n":
		return true
	default:
		return false
	}
}

func executeOperation(ctx context.Context, state ConfirmState, execMap map[OperationKind]executeFn) error {
	fn, ok := execMap[state.Operation]
	if !ok {
		return fmt.Errorf("workflows.destructive_confirm: operation kind desconhecida: %d", state.Operation)
	}
	return fn(ctx, state)
}

func executeRegister(ctx context.Context, state ConfirmState, ledger interfaces.TransactionsLedger, categories interfaces.CategoriesReader) error {
	if state.UpdatePayload == "" {
		return fmt.Errorf("workflows.destructive_confirm.register: update payload ausente")
	}
	candidates, err := categories.SearchDictionary(ctx, state.ResumeText, "expense")
	if err != nil || len(candidates) == 0 {
		return fmt.Errorf("workflows.destructive_confirm.register: categoria não encontrada para %q: %w", state.ResumeText, err)
	}
	switch state.TargetKind {
	case "card_purchase":
		var draft interfaces.RawCardPurchase
		if err := json.Unmarshal([]byte(state.UpdatePayload), &draft); err != nil {
			return fmt.Errorf("workflows.destructive_confirm.register: decode card_purchase: %w", err)
		}
		draft.CategoryID = candidates[0].CategoryID
		_, err = ledger.CreateCardPurchase(ctx, draft)
		return err
	default:
		var draft interfaces.RawTransaction
		if err := json.Unmarshal([]byte(state.UpdatePayload), &draft); err != nil {
			return fmt.Errorf("workflows.destructive_confirm.register: decode transaction: %w", err)
		}
		draft.CategoryID = candidates[0].CategoryID
		_, err = ledger.CreateTransaction(ctx, draft)
		return err
	}
}

func executeDeleteEntry(ctx context.Context, state ConfirmState, ledger interfaces.TransactionsLedger) error {
	id, err := uuid.Parse(state.TargetRef)
	if err != nil {
		return fmt.Errorf("workflows.destructive_confirm.delete_entry: parse uuid: %w", err)
	}
	ref := interfaces.EntryRef{ID: id, Kind: state.TargetKind}
	switch state.TargetKind {
	case "card_purchase":
		return ledger.DeleteCardPurchase(ctx, ref, state.Version)
	default:
		return ledger.DeleteTransaction(ctx, ref, state.Version)
	}
}

func executeEditEntry(ctx context.Context, state ConfirmState, ledger interfaces.TransactionsLedger) error {
	id, err := uuid.Parse(state.TargetRef)
	if err != nil {
		return fmt.Errorf("workflows.destructive_confirm.edit_entry: parse uuid: %w", err)
	}
	if state.UpdatePayload == "" {
		return fmt.Errorf("workflows.destructive_confirm.edit_entry: update payload ausente")
	}
	switch state.TargetKind {
	case "card_purchase":
		var upd interfaces.RawUpdateCardPurchase
		if err := json.Unmarshal([]byte(state.UpdatePayload), &upd); err != nil {
			return fmt.Errorf("workflows.destructive_confirm.edit_entry: decode card_purchase payload: %w", err)
		}
		upd.ID = id
		upd.Version = state.Version
		_, err = ledger.UpdateCardPurchase(ctx, upd)
		return err
	default:
		var upd interfaces.RawUpdateTransaction
		if err := json.Unmarshal([]byte(state.UpdatePayload), &upd); err != nil {
			return fmt.Errorf("workflows.destructive_confirm.edit_entry: decode transaction payload: %w", err)
		}
		upd.ID = id
		upd.Version = state.Version
		_, err = ledger.UpdateTransaction(ctx, upd)
		return err
	}
}

func executeDeleteCard(ctx context.Context, state ConfirmState, cards interfaces.CardManager) error {
	cardID, err := uuid.Parse(state.TargetRef)
	if err != nil {
		return fmt.Errorf("workflows.destructive_confirm.delete_card: parse card uuid: %w", err)
	}
	return cards.SoftDeleteCard(ctx, cardID, state.UserID)
}

func executeUpdateRecurrence(ctx context.Context, state ConfirmState, recurrences interfaces.RecurrenceManager) error {
	if state.UpdatePayload == "" {
		return fmt.Errorf("workflows.destructive_confirm.update_recurrence: update payload ausente")
	}
	var upd interfaces.RawUpdateRecurrence
	if err := json.Unmarshal([]byte(state.UpdatePayload), &upd); err != nil {
		return fmt.Errorf("workflows.destructive_confirm.update_recurrence: decode payload: %w", err)
	}
	_, err := recurrences.UpdateRecurrence(ctx, state.TargetRef, upd)
	return err
}

func executeDeleteRecurrence(ctx context.Context, state ConfirmState, recurrences interfaces.RecurrenceManager) error {
	return recurrences.DeleteRecurrence(ctx, state.TargetRef, state.Version)
}

func executeUpdateCard(ctx context.Context, state ConfirmState, cards interfaces.CardManager) error {
	cardID, err := uuid.Parse(state.TargetRef)
	if err != nil {
		return fmt.Errorf("workflows.destructive_confirm.update_card: parse card uuid: %w", err)
	}
	if state.UpdatePayload == "" {
		return fmt.Errorf("workflows.destructive_confirm.update_card: update payload ausente")
	}
	var upd interfaces.CardUpdate
	if err := json.Unmarshal([]byte(state.UpdatePayload), &upd); err != nil {
		return fmt.Errorf("workflows.destructive_confirm.update_card: decode payload: %w", err)
	}
	upd.ID = cardID
	upd.UserID = state.UserID
	_, err = cards.UpdateCard(ctx, upd)
	return err
}

func BuildImpactNote(ctx context.Context, targetRef, targetKind string, userID uuid.UUID, cards interfaces.CardManager) string {
	switch targetKind {
	case "card":
		id, err := uuid.Parse(targetRef)
		if err != nil {
			return "Remoção permanente do cartão."
		}
		hasOpen, err := cards.HasOpenInstallments(ctx, id, userID)
		if err != nil || !hasOpen {
			return "Remoção permanente do cartão."
		}
		return "⚠️ Este cartão possui compras parceladas em aberto. Removê-lo deixará as parcelas sem cartão associado."
	case "card_purchase":
		return "⚠️ Todas as parcelas desta compra serão removidas."
	default:
		return "Este lançamento será removido permanentemente."
	}
}
