package usecases

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/budgetdraft"
)

type budgetDraftEnvelope struct {
	Kind        string         `json:"kind"`
	TotalCents  int64          `json:"total_cents"`
	Allocations map[string]int `json:"allocations"`
	Competence  string         `json:"competence"`
}

func IsBudgetConfigPending(pendingAction []byte) bool {
	trimmed := strings.TrimSpace(string(pendingAction))
	if trimmed == "" || trimmed == "{}" {
		return false
	}
	var env budgetDraftEnvelope
	if err := json.Unmarshal([]byte(trimmed), &env); err != nil {
		return false
	}
	return env.Kind == budgetdraft.PendingActionKind
}

func DecodeBudgetDraft(pendingAction []byte) (budgetdraft.Draft, error) {
	var env budgetDraftEnvelope
	if err := json.Unmarshal(pendingAction, &env); err != nil {
		return budgetdraft.Draft{}, fmt.Errorf("agent.usecase.budget_draft: decode pending action: %w", err)
	}
	if env.Kind != budgetdraft.PendingActionKind {
		return budgetdraft.Draft{}, fmt.Errorf("agent.usecase.budget_draft: pending action kind %q inesperado", env.Kind)
	}
	draft, err := budgetdraft.Restore(env.TotalCents, env.Allocations, env.Competence)
	if err != nil {
		return budgetdraft.Draft{}, fmt.Errorf("agent.usecase.budget_draft: restore: %w", err)
	}
	return draft, nil
}

func EncodeBudgetDraft(draft budgetdraft.Draft) ([]byte, error) {
	env := budgetDraftEnvelope{
		Kind:        budgetdraft.PendingActionKind,
		TotalCents:  draft.TotalCents(),
		Allocations: draft.Allocations(),
		Competence:  draft.Competence(),
	}
	raw, err := json.Marshal(env)
	if err != nil {
		return nil, fmt.Errorf("agent.usecase.budget_draft: encode pending action: %w", err)
	}
	return raw, nil
}
