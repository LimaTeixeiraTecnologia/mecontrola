package binding

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow/steps"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/budgetdraft"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/confirmation"
)

func parseHITLUserID(raw string) (uuid.UUID, error) {
	return uuid.Parse(strings.TrimSpace(raw))
}

func NewLastTransactionDeleterResolver(lister tools.TransactionLister) steps.TargetResolver {
	return func(ctx context.Context, state confirmation.ConfirmState) (confirmation.ConfirmState, error) {
		result, err := lister.Execute(ctx, tools.TransactionListInput{UserID: state.UserID, RefMonth: ""})
		if err != nil {
			return state, fmt.Errorf("hitl: delete_last resolver: list: %w", err)
		}
		if len(result.Transactions) == 0 {
			state.ShortCircuit = true
			state.Reply = "Não há lançamentos para apagar."
			state.Outcome = int(tools.OutcomeRouted)
			return state, nil
		}
		last := result.Transactions[0]
		state.PromptText = fmt.Sprintf(
			"Você deseja apagar o último lançamento: *%s* de R$ %.2f? Responda *sim* para confirmar ou *não* para cancelar.",
			last.Description,
			float64(last.AmountCents)/100,
		)
		return state, nil
	}
}

func NewLastTransactionDeleterExecutor(deleter tools.LastTransactionDeleter, lister tools.TransactionLister) steps.DestructiveExecutor {
	return func(ctx context.Context, state confirmation.ConfirmState) (confirmation.ConfirmState, error) {
		result, err := lister.Execute(ctx, tools.TransactionListInput{UserID: state.UserID, RefMonth: ""})
		if err != nil {
			return state, fmt.Errorf("hitl: delete_last executor: list: %w", err)
		}
		if len(result.Transactions) == 0 {
			state.ShortCircuit = true
			state.Reply = "Não há lançamentos para apagar."
			state.Outcome = int(tools.OutcomeRouted)
			return state, nil
		}
		last := result.Transactions[0]
		if err := deleter.Execute(ctx, state.UserID, last.ID, last.Version); err != nil {
			return state, fmt.Errorf("hitl: delete_last executor: delete: %w", err)
		}
		state.Outcome = int(tools.OutcomeRouted)
		return state, nil
	}
}

func NewLastTransactionEditorResolver(lister tools.TransactionLister) steps.TargetResolver {
	return func(ctx context.Context, state confirmation.ConfirmState) (confirmation.ConfirmState, error) {
		result, err := lister.Execute(ctx, tools.TransactionListInput{UserID: state.UserID, RefMonth: ""})
		if err != nil {
			return state, fmt.Errorf("hitl: edit_last resolver: list: %w", err)
		}
		if len(result.Transactions) == 0 {
			state.ShortCircuit = true
			state.Reply = "Não há lançamentos para editar."
			state.Outcome = int(tools.OutcomeRouted)
			return state, nil
		}
		last := result.Transactions[0]
		state.PromptText = fmt.Sprintf(
			"Você deseja atualizar o último lançamento: *%s* de R$ %.2f? Responda *sim* para confirmar ou *não* para cancelar.",
			last.Description,
			float64(last.AmountCents)/100,
		)
		return state, nil
	}
}

func NewLastTransactionEditorExecutor(editor tools.LastTransactionEditor, lister tools.TransactionLister) steps.DestructiveExecutor {
	return func(ctx context.Context, state confirmation.ConfirmState) (confirmation.ConfirmState, error) {
		result, err := lister.Execute(ctx, tools.TransactionListInput{UserID: state.UserID, RefMonth: ""})
		if err != nil {
			return state, fmt.Errorf("hitl: edit_last executor: list: %w", err)
		}
		if len(result.Transactions) == 0 {
			state.ShortCircuit = true
			state.Reply = "Não há lançamentos para editar."
			state.Outcome = int(tools.OutcomeRouted)
			return state, nil
		}
		last := result.Transactions[0]
		_, execErr := editor.Execute(ctx, tools.EditTransactionInput{
			UserID:  state.UserID,
			Current: tools.TransactionView{ID: last.ID, Version: last.Version},
		})
		if execErr != nil {
			return state, fmt.Errorf("hitl: edit_last executor: edit: %w", execErr)
		}
		state.Outcome = int(tools.OutcomeRouted)
		return state, nil
	}
}

func NewCardDeleterResolver() steps.TargetResolver {
	return func(_ context.Context, state confirmation.ConfirmState) (confirmation.ConfirmState, error) {
		state.PromptText = "Você deseja remover o cartão? Responda *sim* para confirmar ou *não* para cancelar."
		return state, nil
	}
}

func NewCardDeleterExecutorFn(deleter tools.CardDeleter) steps.DestructiveExecutor {
	return func(ctx context.Context, state confirmation.ConfirmState) (confirmation.ConfirmState, error) {
		uid, err := parseHITLUserID(state.UserID)
		if err != nil {
			return state, fmt.Errorf("hitl: delete_card executor: user_id: %w", err)
		}
		_, execErr := deleter.Execute(ctx, uid, "")
		if execErr != nil {
			return state, fmt.Errorf("hitl: delete_card executor: %w", execErr)
		}
		state.Outcome = int(tools.OutcomeRouted)
		return state, nil
	}
}

func NewBudgetCommitResolver() steps.TargetResolver {
	return func(_ context.Context, state confirmation.ConfirmState) (confirmation.ConfirmState, error) {
		state.PromptText = "Você deseja confirmar a configuração do orçamento? Responda *sim* para confirmar ou *não* para cancelar."
		return state, nil
	}
}

func NewBudgetCommitExecutor(committer tools.BudgetConfigCommitter) steps.DestructiveExecutor {
	return func(ctx context.Context, state confirmation.ConfirmState) (confirmation.ConfirmState, error) {
		uid, err := parseHITLUserID(state.UserID)
		if err != nil {
			return state, fmt.Errorf("hitl: budget_commit executor: user_id: %w", err)
		}
		reply, execErr := committer.Commit(ctx, uid, budgetdraft.Draft{})
		if execErr != nil {
			return state, fmt.Errorf("hitl: budget_commit executor: %w", execErr)
		}
		state.Reply = reply
		state.Outcome = int(tools.OutcomeRouted)
		return state, nil
	}
}
