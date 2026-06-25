package binding

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow/steps"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/confirmation"
	cardinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	transactionsinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
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
		state.TargetTransactionID = last.ID
		state.TargetTransactionVersion = last.Version
		state.PromptText = fmt.Sprintf(
			"Você deseja apagar o último lançamento: *%s* de R$ %.2f? Responda *sim* para confirmar ou *não* para cancelar.",
			last.Description,
			float64(last.AmountCents)/100,
		)
		return state, nil
	}
}

func NewLastTransactionDeleterExecutor(deleter tools.LastTransactionDeleter) steps.DestructiveExecutor {
	return func(ctx context.Context, state confirmation.ConfirmState) (confirmation.ConfirmState, error) {
		if state.TargetTransactionID == "" {
			return state, fmt.Errorf("hitl: delete_last executor: no target transaction captured")
		}
		if err := deleter.Execute(ctx, state.UserID, state.TargetTransactionID, state.TargetTransactionVersion); err != nil {
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
		state.TargetTransactionID = last.ID
		state.TargetTransactionVersion = last.Version
		state.PromptText = fmt.Sprintf(
			"Você deseja atualizar o último lançamento: *%s* de R$ %.2f? Responda *sim* para confirmar ou *não* para cancelar.",
			last.Description,
			float64(last.AmountCents)/100,
		)
		return state, nil
	}
}

func NewLastTransactionEditorExecutor(editor tools.LastTransactionEditor) steps.DestructiveExecutor {
	return func(ctx context.Context, state confirmation.ConfirmState) (confirmation.ConfirmState, error) {
		if state.TargetTransactionID == "" {
			return state, fmt.Errorf("hitl: edit_last executor: no target transaction captured")
		}
		_, execErr := editor.Execute(ctx, tools.EditTransactionInput{
			UserID:    state.UserID,
			Current:   tools.TransactionView{ID: state.TargetTransactionID, Version: state.TargetTransactionVersion},
			NewAmount: state.NewAmountCents,
		})
		if execErr != nil {
			return state, fmt.Errorf("hitl: edit_last executor: edit: %w", execErr)
		}
		state.Outcome = int(tools.OutcomeRouted)
		return state, nil
	}
}

func NewDeleteByRefResolver() steps.TargetResolver {
	return func(_ context.Context, state confirmation.ConfirmState) (confirmation.ConfirmState, error) {
		if state.TargetTransactionID == "" {
			return state, fmt.Errorf("hitl: delete_by_ref resolver: no target selected")
		}
		state.PromptText = fmt.Sprintf(
			"Você deseja apagar o lançamento: *%s* de %s? Responda *sim* para confirmar ou *não* para cancelar.",
			state.TargetDescription,
			tools.FormatBRL(state.TargetAmountCents),
		)
		return state, nil
	}
}

func NewDeleteByRefExecutor(deleter tools.LastTransactionDeleter) steps.DestructiveExecutor {
	return func(ctx context.Context, state confirmation.ConfirmState) (confirmation.ConfirmState, error) {
		if state.TargetTransactionID == "" {
			return state, fmt.Errorf("hitl: delete_by_ref executor: no target transaction captured")
		}
		if err := deleter.Execute(ctx, state.UserID, state.TargetTransactionID, state.TargetTransactionVersion); err != nil {
			if reply, ok := versionConflictReply(err); ok {
				state.ShortCircuit = true
				state.Reply = reply
				state.Outcome = int(tools.OutcomeRouted)
				return state, nil
			}
			return state, fmt.Errorf("hitl: delete_by_ref executor: delete: %w", err)
		}
		state.Outcome = int(tools.OutcomeRouted)
		return state, nil
	}
}

func NewEditByRefResolver() steps.TargetResolver {
	return func(_ context.Context, state confirmation.ConfirmState) (confirmation.ConfirmState, error) {
		if state.TargetTransactionID == "" {
			return state, fmt.Errorf("hitl: edit_by_ref resolver: no target selected")
		}
		state.PromptText = fmt.Sprintf(
			"Você deseja atualizar *%s* de %s para *%s*? Responda *sim* para confirmar ou *não* para cancelar.",
			state.TargetDescription,
			tools.FormatBRL(state.TargetAmountCents),
			tools.FormatBRL(state.NewAmountCents),
		)
		return state, nil
	}
}

func NewEditByRefExecutor(editor tools.LastTransactionEditor) steps.DestructiveExecutor {
	return func(ctx context.Context, state confirmation.ConfirmState) (confirmation.ConfirmState, error) {
		if state.TargetTransactionID == "" {
			return state, fmt.Errorf("hitl: edit_by_ref executor: no target transaction captured")
		}
		_, execErr := editor.Execute(ctx, tools.EditTransactionInput{
			UserID:    state.UserID,
			Current:   tools.TransactionView{ID: state.TargetTransactionID, Version: state.TargetTransactionVersion},
			NewAmount: state.NewAmountCents,
		})
		if execErr != nil {
			if reply, ok := versionConflictReply(execErr); ok {
				state.ShortCircuit = true
				state.Reply = reply
				state.Outcome = int(tools.OutcomeRouted)
				return state, nil
			}
			return state, fmt.Errorf("hitl: edit_by_ref executor: edit: %w", execErr)
		}
		state.Outcome = int(tools.OutcomeRouted)
		return state, nil
	}
}

func versionConflictReply(err error) (string, bool) {
	if errors.Is(err, transactionsinterfaces.ErrTransactionVersionConflict) {
		return "Esse lançamento mudou desde que você buscou. Tente de novo, por favor. 🙏", true
	}
	return "", false
}

func NewCardDeleterResolver(lister tools.CardLister) steps.TargetResolver {
	return func(ctx context.Context, state confirmation.ConfirmState) (confirmation.ConfirmState, error) {
		uid, err := parseHITLUserID(state.UserID)
		if err != nil {
			return state, fmt.Errorf("hitl: delete_card resolver: user_id: %w", err)
		}
		cards, err := lister.Execute(ctx, cardinput.ListCards{UserID: uid, Limit: defaultListCardsLimit})
		if err != nil {
			return state, fmt.Errorf("hitl: delete_card resolver: list: %w", err)
		}
		resolved, err := resolveCardExact(cards, state.CardName)
		if err != nil {
			state.ShortCircuit = true
			state.Reply = fmt.Sprintf("Não encontrei o cartão '%s'.", state.CardName)
			state.Outcome = int(tools.OutcomeRouted)
			return state, nil
		}
		label := strings.TrimSpace(resolved.Nickname)
		if label == "" {
			label = strings.TrimSpace(resolved.Name)
		}
		state.PromptText = fmt.Sprintf(
			"Você deseja remover o cartão *%s*? Responda *sim* para confirmar ou *não* para cancelar.",
			label,
		)
		return state, nil
	}
}

func NewCardDeleterExecutorFn(deleter tools.CardDeleter) steps.DestructiveExecutor {
	return func(ctx context.Context, state confirmation.ConfirmState) (confirmation.ConfirmState, error) {
		uid, err := parseHITLUserID(state.UserID)
		if err != nil {
			return state, fmt.Errorf("hitl: delete_card executor: user_id: %w", err)
		}
		_, execErr := deleter.Execute(ctx, uid, state.CardName)
		if execErr != nil {
			return state, fmt.Errorf("hitl: delete_card executor: %w", execErr)
		}
		state.Outcome = int(tools.OutcomeRouted)
		return state, nil
	}
}

func NewBudgetCommitResolver() steps.TargetResolver {
	return func(_ context.Context, state confirmation.ConfirmState) (confirmation.ConfirmState, error) {
		draft, err := state.BudgetDraft()
		if err != nil {
			state.ShortCircuit = true
			state.Reply = "Não consegui carregar o rascunho do orçamento. Tente configurar novamente."
			state.Outcome = int(tools.OutcomeRouted)
			return state, nil
		}
		state.PromptText = fmt.Sprintf(
			"Você deseja ativar o orçamento de *R$ %.2f* (%s) com as alocações abaixo? Responda *sim* para confirmar ou *não* para cancelar.\n\n%s",
			float64(draft.TotalCents())/100,
			draft.Competence(),
			formatBudgetAllocations(draft.Allocations()),
		)
		return state, nil
	}
}

func formatBudgetAllocations(allocations map[string]int) string {
	if len(allocations) == 0 {
		return "_sem alocações_"
	}
	var parts []string
	for slug, cents := range allocations {
		parts = append(parts, fmt.Sprintf("• %s: R$ %.2f", slug, float64(cents)/100))
	}
	return strings.Join(parts, "\n")
}

func NewBudgetCommitExecutor(committer tools.BudgetConfigCommitter) steps.DestructiveExecutor {
	return func(ctx context.Context, state confirmation.ConfirmState) (confirmation.ConfirmState, error) {
		uid, err := parseHITLUserID(state.UserID)
		if err != nil {
			return state, fmt.Errorf("hitl: budget_commit executor: user_id: %w", err)
		}
		draft, draftErr := state.BudgetDraft()
		if draftErr != nil {
			return state, fmt.Errorf("hitl: budget_commit executor: draft: %w", draftErr)
		}
		reply, execErr := committer.Commit(ctx, uid, draft)
		if execErr != nil {
			return state, fmt.Errorf("hitl: budget_commit executor: %w", execErr)
		}
		state.Reply = reply
		state.Outcome = int(tools.OutcomeRouted)
		return state, nil
	}
}
