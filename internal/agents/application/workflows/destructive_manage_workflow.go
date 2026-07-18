package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

const (
	DestructiveManageWorkflowID  = "destructive-confirm"
	stepDestructiveManageID      = "destructive-confirm"
	DestructiveManageStaleAfter  = 20 * time.Minute
	DestructiveManageReaperBatch = 100
)

func DestructiveManageKey(resourceID, threadID string) string {
	return CorrelationKey(resourceID, threadID, DestructiveManageWorkflowID)
}

func BuildDestructiveManageWorkflow(cards interfaces.CardManager, recurrences interfaces.RecurrenceManager, ledger interfaces.TransactionsLedger) workflow.Definition[DestructiveManageState] {
	step := workflow.NewStepFunc(stepDestructiveManageID, buildDestructiveManageStep(cards, recurrences, ledger))
	return workflow.Definition[DestructiveManageState]{
		ID:          DestructiveManageWorkflowID,
		Root:        step,
		Durable:     true,
		MaxAttempts: 1,
	}
}

func BuildDestructiveManageReaper(store workflow.Store, o11y observability.Observability) *workflow.StaleSuspendedReaper {
	return workflow.NewStaleSuspendedReaper(store, DestructiveManageWorkflowID, DestructiveManageStaleAfter, DestructiveManageReaperBatch, o11y)
}

type destructiveManageExecFn func(ctx context.Context, state DestructiveManageState) error

func destructiveManageExecMap(cards interfaces.CardManager, recurrences interfaces.RecurrenceManager, ledger interfaces.TransactionsLedger) map[DestructiveOperationKind]destructiveManageExecFn {
	return map[DestructiveOperationKind]destructiveManageExecFn{
		DestructiveOpDeleteCard: func(ctx context.Context, state DestructiveManageState) error {
			return executeDestructiveManageDeleteCard(ctx, state, cards)
		},
		DestructiveOpDeleteRecurrence: func(ctx context.Context, state DestructiveManageState) error {
			return executeDestructiveManageDeleteRecurrence(ctx, state, recurrences)
		},
		DestructiveOpDeleteEntry: func(ctx context.Context, state DestructiveManageState) error {
			return executeDestructiveManageDeleteEntry(ctx, state, ledger)
		},
		DestructiveOpUpdateRecurrence: func(ctx context.Context, state DestructiveManageState) error {
			return executeDestructiveManageUpdateRecurrence(ctx, state, recurrences)
		},
	}
}

func buildDestructiveManageStep(cards interfaces.CardManager, recurrences interfaces.RecurrenceManager, ledger interfaces.TransactionsLedger) func(context.Context, DestructiveManageState) (workflow.StepOutput[DestructiveManageState], error) {
	execMap := destructiveManageExecMap(cards, recurrences, ledger)
	return func(ctx context.Context, state DestructiveManageState) (workflow.StepOutput[DestructiveManageState], error) {
		if state.ResumeText == "" {
			state.ResponseText = destructiveManageConfirmQuestion(state)
			return destructiveManageSuspend(state, state.ResponseText), nil
		}

		if isDestructiveManageExpired(state, time.Now().UTC()) {
			state.Status = DestructiveManageExpired
			state.Expired = true
			state.ResponseText = ""
			return workflow.StepOutput[DestructiveManageState]{State: state, Status: workflow.StepStatusCompleted}, nil
		}

		msg := PendingMessage{Text: state.ResumeText, MessageID: state.IncomingMessageID}
		action := DecideDestructiveManageConfirmation(state, msg, time.Now().UTC())
		state.ResumeText = ""

		switch action {
		case DestructiveManageActionAccept:
			state.MessageID = state.IncomingMessageID
			if err := destructiveManageExecute(ctx, state, execMap); err != nil {
				state.ResponseText = fmt.Sprintf("❌ Não foi possível realizar a operação: %s", err.Error())
				return workflow.StepOutput[DestructiveManageState]{State: state, Status: workflow.StepStatusFailed}, fmt.Errorf("workflows.destructive_manage: execute: %w", err)
			}
			state.Status = DestructiveManageCompleted
			state.ResponseText = destructiveManageSuccessMessage(state.Operation)
			return workflow.StepOutput[DestructiveManageState]{State: state, Status: workflow.StepStatusCompleted}, nil
		case DestructiveManageActionCancel:
			state.Status = DestructiveManageCancelled
			state.ResponseText = "🚫 Operação cancelada conforme solicitado."
			return workflow.StepOutput[DestructiveManageState]{State: state, Status: workflow.StepStatusCompleted}, nil
		case DestructiveManageActionExpire:
			state.Status = DestructiveManageExpired
			state.Expired = true
			state.ResponseText = ""
			return workflow.StepOutput[DestructiveManageState]{State: state, Status: workflow.StepStatusCompleted}, nil
		default:
			state.RepromptDone = true
			state.ResponseText = "Não entendi. Por favor, responda apenas *sim* ou *não* para confirmar a operação."
			return destructiveManageSuspend(state, state.ResponseText), nil
		}
	}
}

func destructiveManageSuspend(state DestructiveManageState, prompt string) workflow.StepOutput[DestructiveManageState] {
	if state.SuspendedAt.IsZero() {
		state.SuspendedAt = time.Now().UTC()
	}
	return workflow.StepOutput[DestructiveManageState]{
		State:  state,
		Status: workflow.StepStatusSuspended,
		Suspend: &workflow.Suspension{
			Reason: workflow.SuspendAwaitingInput,
			Prompt: prompt,
		},
	}
}

func destructiveManageConfirmQuestion(state DestructiveManageState) string {
	base := "⚠️ Você confirma esta operação?\n\n" + state.ImpactNote
	return base + "\n\nResponda *sim* para confirmar ou *não* para cancelar."
}

func destructiveManageSuccessMessage(op DestructiveOperationKind) string {
	switch op {
	case DestructiveOpDeleteCard:
		return "✅ 💳 removido com sucesso."
	case DestructiveOpDeleteRecurrence:
		return "✅ Recorrência removida com sucesso."
	case DestructiveOpDeleteEntry:
		return "✅ Lançamento removido com sucesso."
	case DestructiveOpUpdateRecurrence:
		return "✅ Recorrência atualizada com sucesso."
	default:
		return "✅ Operação realizada com sucesso."
	}
}

func destructiveManageExecute(ctx context.Context, state DestructiveManageState, execMap map[DestructiveOperationKind]destructiveManageExecFn) error {
	fn, ok := execMap[state.Operation]
	if !ok {
		return fmt.Errorf("workflows.destructive_manage: operation kind desconhecida: %d", state.Operation)
	}
	return fn(ctx, state)
}

func executeDestructiveManageDeleteCard(ctx context.Context, state DestructiveManageState, cards interfaces.CardManager) error {
	cardID, err := uuid.Parse(state.TargetRef)
	if err != nil {
		return fmt.Errorf("workflows.destructive_manage.delete_card: parse card uuid: %w", err)
	}
	return cards.SoftDeleteCard(ctx, cardID, state.UserID)
}

func executeDestructiveManageDeleteRecurrence(ctx context.Context, state DestructiveManageState, recurrences interfaces.RecurrenceManager) error {
	return recurrences.DeleteRecurrence(ctx, state.TargetRef, state.Version)
}

func executeDestructiveManageDeleteEntry(ctx context.Context, state DestructiveManageState, ledger interfaces.TransactionsLedger) error {
	entryID, err := uuid.Parse(state.TargetRef)
	if err != nil {
		return fmt.Errorf("workflows.destructive_manage.delete_entry: parse entry uuid: %w", err)
	}
	return ledger.DeleteTransaction(ctx, interfaces.EntryRef{ID: entryID}, state.Version)
}

func executeDestructiveManageUpdateRecurrence(ctx context.Context, state DestructiveManageState, recurrences interfaces.RecurrenceManager) error {
	var upd interfaces.RawUpdateRecurrence
	if err := json.Unmarshal([]byte(state.UpdatePayload), &upd); err != nil {
		return fmt.Errorf("workflows.destructive_manage.update_recurrence: unmarshal payload: %w", err)
	}
	upd.Version = state.Version
	_, err := recurrences.UpdateRecurrence(ctx, state.TargetRef, upd)
	return err
}

func BuildDestructiveManageImpactNote(ctx context.Context, targetRef, targetKind string, userID uuid.UUID, cards interfaces.CardManager) string {
	switch targetKind {
	case "card":
		id, err := uuid.Parse(targetRef)
		if err != nil {
			return "Remoção permanente do 💳."
		}
		hasOpen, err := cards.HasOpenInstallments(ctx, id, userID)
		if err != nil || !hasOpen {
			return "Remoção permanente do 💳."
		}
		return "⚠️ Este 💳 possui compras parceladas em aberto. Removê-lo deixará as parcelas sem 💳 associado."
	default:
		return "Esta recorrência será removida permanentemente."
	}
}

func ContinueDestructiveManage(
	ctx context.Context,
	engine workflow.Engine[DestructiveManageState],
	def workflow.Definition[DestructiveManageState],
	key string,
	userMessage string,
	messageID string,
) (bool, string, error) {
	resumeBytes, err := json.Marshal(map[string]string{"resumeText": userMessage, "incomingMessageId": messageID})
	if err != nil {
		return false, "", fmt.Errorf("workflows.destructive_manage: marshal resume: %w", err)
	}

	result, resumeErr := engine.Resume(ctx, def, key, resumeBytes)
	if result.Status == 0 && resumeErr == nil {
		return false, "", nil
	}

	if resumeErr != nil {
		return true, result.State.ResponseText, fmt.Errorf("workflows.destructive_manage: resume: %w", resumeErr)
	}

	if result.State.Expired {
		return false, "", nil
	}

	return true, result.State.ResponseText, nil
}
