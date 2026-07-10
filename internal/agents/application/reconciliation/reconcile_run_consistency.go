package reconciliation

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type RunConsistencyRow struct {
	RunID             string
	CorrelationKey    string
	RunStatus         agent.RunStatus
	Outcome           agent.ToolOutcome
	LedgerWrites      int
	TransactionCount  int
	WorkflowStatus    workflow.RunStatus
	WorkflowStateSet  bool
	WorkflowStateText string
	WorkflowFound     bool
}

type ViolationKind string

const (
	ViolationEmptyCorrelationKey     ViolationKind = "empty_correlation_key"
	ViolationSucceededNoEffect       ViolationKind = "succeeded_without_effect"
	ViolationFailedOrphanWrite       ViolationKind = "failed_with_orphan_write"
	ViolationStatusDivergence        ViolationKind = "status_divergence"
	ViolationWorkflowStateDivergence ViolationKind = "workflow_state_divergence"
)

type Violation struct {
	RunID string
	Kind  ViolationKind
}

type RunConsistencyReader interface {
	RunConsistencyRows(ctx context.Context, agentID string, since time.Time) ([]RunConsistencyRow, error)
}

type ReconcileRunConsistency struct {
	reader RunConsistencyReader
	o11y   observability.Observability
}

func NewReconcileRunConsistency(
	reader RunConsistencyReader,
	o11y observability.Observability,
) *ReconcileRunConsistency {
	return &ReconcileRunConsistency{
		reader: reader,
		o11y:   o11y,
	}
}

func (u *ReconcileRunConsistency) Execute(ctx context.Context, agentID string, since time.Time) ([]Violation, error) {
	ctx, span := u.o11y.Tracer().Start(ctx, "agents.usecase.reconcile_run_consistency")
	defer span.End()

	rows, err := u.reader.RunConsistencyRows(ctx, agentID, since)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("agents.usecase.reconcile_run_consistency: %w", err)
	}

	return DecideViolations(rows), nil
}

func DecideViolations(rows []RunConsistencyRow) []Violation {
	var violations []Violation
	for _, row := range rows {
		violations = append(violations, decideRowViolations(row)...)
	}
	return violations
}

func decideRowViolations(row RunConsistencyRow) []Violation {
	var violations []Violation

	if row.CorrelationKey == "" {
		violations = append(violations, Violation{RunID: row.RunID, Kind: ViolationEmptyCorrelationKey})
	}

	hasEffect := row.LedgerWrites > 0 || row.TransactionCount > 0
	workflowSucceeded := row.WorkflowFound && row.WorkflowStatus == workflow.RunStatusSucceeded

	if isSuccessfulWrite(row) && !hasEffect && !workflowSucceeded {
		violations = append(violations, Violation{RunID: row.RunID, Kind: ViolationSucceededNoEffect})
	}

	if row.RunStatus == agent.RunStatusFailed && hasEffect {
		violations = append(violations, Violation{RunID: row.RunID, Kind: ViolationFailedOrphanWrite})
	}

	if row.WorkflowFound && statusDiverges(row.RunStatus, row.WorkflowStatus) {
		violations = append(violations, Violation{RunID: row.RunID, Kind: ViolationStatusDivergence})
	}

	if row.WorkflowFound && workflowStateDiverges(row) {
		violations = append(violations, Violation{RunID: row.RunID, Kind: ViolationWorkflowStateDivergence})
	}

	return violations
}

func workflowStateDiverges(row RunConsistencyRow) bool {
	if !row.WorkflowStateSet || row.WorkflowStateText == "" {
		return false
	}
	if row.WorkflowStatus == workflow.RunStatusRunning || row.WorkflowStatus == workflow.RunStatusSuspended {
		return false
	}
	return row.WorkflowStateText != row.WorkflowStatus.String()
}

func isSuccessfulWrite(row RunConsistencyRow) bool {
	return row.RunStatus == agent.RunStatusSucceeded && row.Outcome == agent.ToolOutcomeRouted
}

func statusDiverges(run agent.RunStatus, wf workflow.RunStatus) bool {
	mapped, ok := mapAgentToWorkflow(run)
	if !ok {
		return false
	}
	if wf == workflow.RunStatusRunning || wf == workflow.RunStatusSuspended {
		return false
	}
	return mapped.String() != wf.String()
}

func mapAgentToWorkflow(run agent.RunStatus) (workflow.RunStatus, bool) {
	switch run {
	case agent.RunStatusSucceeded:
		return workflow.RunStatusSucceeded, true
	case agent.RunStatusFailed:
		return workflow.RunStatusFailed, true
	default:
		return 0, false
	}
}
