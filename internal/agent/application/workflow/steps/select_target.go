package steps

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/confirmation"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type selectOutcome string

const (
	selectOutcomeNone     selectOutcome = "none"
	selectOutcomeFound    selectOutcome = "found"
	selectOutcomeMulti    selectOutcome = "multi"
	selectOutcomeReprompt selectOutcome = "reprompt"
	selectOutcomeCancel   selectOutcome = "cancel"
)

type selectDecision struct {
	output  platform.StepOutput[confirmation.ConfirmState]
	outcome selectOutcome
	emit    bool
}

type selectTargetStep struct {
	counter observability.Counter
}

func NewSelectTarget() platform.Step[confirmation.ConfirmState] {
	return &selectTargetStep{}
}

func NewSelectTargetWithObservability(o11y observability.Observability) platform.Step[confirmation.ConfirmState] {
	if o11y == nil {
		return &selectTargetStep{}
	}
	counter := o11y.Metrics().Counter("agent_target_select_total", "Total by-ref target selections by outcome", "1")
	return &selectTargetStep{counter: counter}
}

func (s *selectTargetStep) ID() string { return "select_target" }

func (s *selectTargetStep) Execute(ctx context.Context, state confirmation.ConfirmState) (platform.StepOutput[confirmation.ConfirmState], error) {
	decision := decideSelectTarget(state)
	if decision.emit && s.counter != nil {
		s.counter.Add(ctx, 1, observability.String("outcome", string(decision.outcome)))
	}
	return decision.output, nil
}

func decideSelectTarget(state confirmation.ConfirmState) selectDecision {
	if state.IsDone() {
		return selectDecision{output: completed(state)}
	}
	if !isByRefOperation(state.OperationKind) {
		return selectDecision{output: completed(state)}
	}

	if state.AwaitingApproval == confirmation.AwaitingSelect {
		return decideSelectionResume(state)
	}

	if len(state.Candidates) == 0 {
		state.ShortCircuit = true
		state.Reply = selectNoneText(state.SearchQuery)
		state.Outcome = int(tools.OutcomeRouted)
		return selectDecision{output: completed(state), outcome: selectOutcomeNone, emit: true}
	}

	if len(state.Candidates) == 1 {
		applyCandidate(&state, state.Candidates[0])
		return selectDecision{output: completed(state), outcome: selectOutcomeFound, emit: true}
	}

	state.AwaitingApproval = confirmation.AwaitingSelect
	return selectDecision{output: suspended(state, selectListText(state.SearchQuery, state.Candidates)), outcome: selectOutcomeMulti, emit: true}
}

func decideSelectionResume(state confirmation.ConfirmState) selectDecision {
	idx, ok := parseSelectionIndex(state.ResumeText, len(state.Candidates))
	if ok {
		state.AwaitingApproval = confirmation.AwaitingNone
		applyCandidate(&state, state.Candidates[idx])
		return selectDecision{output: completed(state), outcome: selectOutcomeFound, emit: true}
	}

	if state.SelectRepromptCount >= 1 {
		state.ShortCircuit = true
		state.AwaitingApproval = confirmation.AwaitingNone
		state.Outcome = int(tools.OutcomeRouted)
		state.Reply = selectCancelledAmbiguousText
		return selectDecision{output: completed(state), outcome: selectOutcomeCancel, emit: true}
	}

	state.SelectRepromptCount++
	return selectDecision{output: suspended(state, selectListText(state.SearchQuery, state.Candidates)), outcome: selectOutcomeReprompt, emit: true}
}

func applyCandidate(state *confirmation.ConfirmState, candidate confirmation.TargetCandidate) {
	state.TargetTransactionID = candidate.TxID
	state.TargetTransactionVersion = candidate.Version
	state.TargetDescription = candidate.Description
	state.TargetAmountCents = candidate.AmountCents
}

func parseSelectionIndex(raw string, total int) (int, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, false
	}
	fields := strings.Fields(trimmed)
	n, err := strconv.Atoi(fields[0])
	if err != nil {
		return 0, false
	}
	if n < 1 || n > total {
		return 0, false
	}
	return n - 1, true
}

func completed(state confirmation.ConfirmState) platform.StepOutput[confirmation.ConfirmState] {
	return platform.StepOutput[confirmation.ConfirmState]{State: state, Status: platform.StepStatusCompleted}
}

func suspended(state confirmation.ConfirmState, prompt string) platform.StepOutput[confirmation.ConfirmState] {
	return platform.StepOutput[confirmation.ConfirmState]{
		State:  state,
		Status: platform.StepStatusSuspended,
		Suspend: &platform.Suspension{
			Reason: platform.SuspendAwaitingInput,
			Prompt: prompt,
		},
	}
}

func selectNoneText(query string) string {
	q := strings.TrimSpace(query)
	if q == "" {
		return "Não encontrei nenhum lançamento com essa descrição."
	}
	return fmt.Sprintf("Não encontrei nenhum lançamento com \"%s\".", q)
}

func selectListText(query string, candidates []confirmation.TargetCandidate) string {
	var b strings.Builder
	q := strings.TrimSpace(query)
	if q == "" {
		b.WriteString("Encontrei mais de um lançamento. Qual deles?\n")
	} else {
		fmt.Fprintf(&b, "Encontrei mais de um lançamento com \"%s\". Qual deles?\n", q)
	}
	for i, candidate := range candidates {
		fmt.Fprintf(&b, "%d) %s — %s (%s)\n", i+1, tools.FormatBRL(candidate.AmountCents), candidate.Description, candidate.OccurredAt)
	}
	b.WriteString("Responda com o número.")
	return b.String()
}

const selectCancelledAmbiguousText = "Não entendi qual lançamento você quer. Operação cancelada por segurança."
