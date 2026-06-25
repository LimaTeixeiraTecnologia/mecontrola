package steps

import (
	"context"
	"fmt"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/confirmation"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type ResolveCandidatesDeps struct {
	Searcher tools.TransactionSearcher
}

type resolveCandidatesStep struct {
	searcher tools.TransactionSearcher
}

func NewResolveCandidates(deps ResolveCandidatesDeps) platform.Step[confirmation.ConfirmState] {
	return &resolveCandidatesStep{searcher: deps.Searcher}
}

func (s *resolveCandidatesStep) ID() string { return "resolve_candidates" }

func (s *resolveCandidatesStep) Execute(ctx context.Context, state confirmation.ConfirmState) (platform.StepOutput[confirmation.ConfirmState], error) {
	if state.IsDone() {
		return platform.StepOutput[confirmation.ConfirmState]{State: state, Status: platform.StepStatusCompleted}, nil
	}
	if !isByRefOperation(state.OperationKind) {
		return platform.StepOutput[confirmation.ConfirmState]{State: state, Status: platform.StepStatusCompleted}, nil
	}
	if len(state.Candidates) > 0 {
		return platform.StepOutput[confirmation.ConfirmState]{State: state, Status: platform.StepStatusCompleted}, nil
	}
	if s.searcher == nil {
		return platform.StepOutput[confirmation.ConfirmState]{State: state, Status: platform.StepStatusFailed}, fmt.Errorf("resolve_candidates: searcher not configured")
	}

	result, err := s.searcher.Execute(ctx, tools.TransactionSearchInput{
		UserID: state.UserID,
		Query:  state.SearchQuery,
	})
	if err != nil {
		return platform.StepOutput[confirmation.ConfirmState]{State: state, Status: platform.StepStatusFailed}, fmt.Errorf("resolve_candidates: %w", err)
	}

	candidates := make([]confirmation.TargetCandidate, 0, len(result.Candidates))
	for _, view := range result.Candidates {
		candidates = append(candidates, confirmation.TargetCandidate{
			TxID:        view.ID,
			Version:     view.Version,
			Description: view.Description,
			AmountCents: view.AmountCents,
			OccurredAt:  view.OccurredAt.Format("02/01"),
		})
	}
	state.Candidates = candidates
	return platform.StepOutput[confirmation.ConfirmState]{State: state, Status: platform.StepStatusCompleted}, nil
}

func isByRefOperation(op confirmation.OperationKind) bool {
	return op == confirmation.OperationDeleteByRef || op == confirmation.OperationEditByRef
}
