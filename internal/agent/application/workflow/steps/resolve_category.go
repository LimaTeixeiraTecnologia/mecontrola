package steps

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/pendingexpense"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type CategoryResolverFunc func(ctx context.Context, state ExpenseState) (ExpenseState, error)

type resolveCategoryStep struct {
	resolver CategoryResolverFunc
}

func NewResolveCategory(resolver CategoryResolverFunc) platform.Step[ExpenseState] {
	return &resolveCategoryStep{resolver: resolver}
}

func (s *resolveCategoryStep) ID() string { return "resolve_category" }

func (s *resolveCategoryStep) Execute(ctx context.Context, state ExpenseState) (platform.StepOutput[ExpenseState], error) {
	if state.IsDone() {
		return platform.StepOutput[ExpenseState]{State: state, Status: platform.StepStatusCompleted}, nil
	}

	if state.AwaitingKind != "" {
		return s.resume(ctx, state)
	}

	resolved, err := s.resolver(ctx, state)
	if err == nil {
		return platform.StepOutput[ExpenseState]{State: resolved, Status: platform.StepStatusCompleted}, nil
	}

	if ambiguous, ok := errors.AsType[*tools.CategoryAmbiguousError](err); ok {
		state.Candidates = ambiguous.Candidates
		state.AwaitingKind = pendingexpense.AwaitingCategoryChoice
		if len(ambiguous.Candidates) > 0 {
			state.CategoryID = ambiguous.Candidates[0]
			state.CategoryPath = ambiguous.Candidates[0]
		}
		state.Outcome = tools.OutcomeClarify
		state.Reply = tools.FormatCategoryAmbiguous(ambiguous.Candidates)
		return platform.StepOutput[ExpenseState]{
			State:  state,
			Status: platform.StepStatusSuspended,
			Suspend: &platform.Suspension{
				Reason: platform.SuspendAwaitingInput,
				Prompt: state.Reply,
			},
		}, nil
	}

	if needsConfirmation, ok := errors.AsType[*tools.CategoryNeedsConfirmationError](err); ok {
		state.Candidates = needsConfirmation.Candidates
		state.AwaitingKind = pendingexpense.AwaitingCategoryConfirm
		if len(needsConfirmation.Candidates) > 0 {
			state.CategoryID = needsConfirmation.Candidates[0]
			state.CategoryPath = needsConfirmation.Candidates[0]
		}
		state.Outcome = tools.OutcomeClarify
		state.Reply = tools.FormatCategoryNeedsConfirmation(needsConfirmation.Candidates)
		return platform.StepOutput[ExpenseState]{
			State:  state,
			Status: platform.StepStatusSuspended,
			Suspend: &platform.Suspension{
				Reason: platform.SuspendAwaitingInput,
				Prompt: state.Reply,
			},
		}, nil
	}

	if errors.Is(err, tools.ErrCategoryNotFound) {
		state.Outcome = tools.OutcomeClarify
		state.Reply = tools.FormatCategoryNotFound(resolveCategoryHint(state))
		state.ShortCircuit = true
		return platform.StepOutput[ExpenseState]{State: state, Status: platform.StepStatusCompleted}, nil
	}

	if errors.Is(err, tools.ErrCategoryHintMissing) {
		state.Outcome = tools.OutcomeClarify
		state.Reply = tools.CategoryNoHintText()
		state.ShortCircuit = true
		return platform.StepOutput[ExpenseState]{State: state, Status: platform.StepStatusCompleted}, nil
	}

	return platform.StepOutput[ExpenseState]{State: state, Status: platform.StepStatusFailed}, err
}

func (s *resolveCategoryStep) resume(_ context.Context, state ExpenseState) (platform.StepOutput[ExpenseState], error) {
	resumeText := strings.TrimSpace(state.ResumeText)

	if matchesExpenseCancellation(resumeText) {
		state.Outcome = tools.OutcomeRouted
		state.Reply = expenseCancelledText
		state.ShortCircuit = true
		state.AwaitingKind = ""
		return platform.StepOutput[ExpenseState]{State: state, Status: platform.StepStatusCompleted}, nil
	}

	if state.AwaitingKind == pendingexpense.AwaitingCategoryChoice {
		matched := matchCandidateByText(resumeText, state.Candidates)
		if matched == "" {
			return platform.StepOutput[ExpenseState]{State: state, Status: platform.StepStatusSuspended,
				Suspend: &platform.Suspension{Reason: platform.SuspendAwaitingInput, Prompt: state.Reply},
			}, nil
		}
		state.CategoryID = matched
		state.CategoryPath = matched
		fc := matched
		state.ForceCategory = &fc
		state.AwaitingKind = ""
		return platform.StepOutput[ExpenseState]{State: state, Status: platform.StepStatusCompleted}, nil
	}

	if matchesExpenseConfirmation(resumeText) {
		fc := state.CategoryID
		state.ForceCategory = &fc
		state.AwaitingKind = ""
		return platform.StepOutput[ExpenseState]{State: state, Status: platform.StepStatusCompleted}, nil
	}

	return platform.StepOutput[ExpenseState]{State: state, Status: platform.StepStatusSuspended,
		Suspend: &platform.Suspension{Reason: platform.SuspendAwaitingInput, Prompt: state.Reply},
	}, nil
}

const expenseCancelledText = "Ok, cancelei o lançamento. Quando quiser registrar, é só me dizer. 😊"

func resolveCategoryHint(state ExpenseState) string {
	hint := strings.TrimSpace(state.CategoryHint)
	if hint == "" {
		hint = strings.TrimSpace(state.Merchant)
	}
	return hint
}

func matchCandidateByText(text string, candidates []string) string {
	normalized := strings.ToLower(strings.TrimSpace(text))
	if normalized == "" {
		return ""
	}
	if idx, err := strconv.Atoi(normalized); err == nil && idx >= 1 && idx <= len(candidates) {
		return candidates[idx-1]
	}
	for _, candidate := range candidates {
		for segment := range strings.SplitSeq(strings.ToLower(candidate), " > ") {
			if strings.HasPrefix(strings.TrimSpace(segment), normalized) {
				return candidate
			}
		}
	}
	return ""
}

func matchesExpenseConfirmation(text string) bool {
	t := strings.ToLower(strings.TrimSpace(text))
	for _, word := range []string{"sim", "s", "confirma", "confirmado", "pode", "ok", "yes"} {
		if t == word || strings.HasPrefix(t, word+",") || strings.HasPrefix(t, word+" ") {
			return true
		}
	}
	return false
}

func matchesExpenseCancellation(text string) bool {
	t := strings.ToLower(strings.TrimSpace(text))
	return t == "não" || t == "nao" || t == "n" || t == "no" || t == "cancela" || t == "cancelar"
}
