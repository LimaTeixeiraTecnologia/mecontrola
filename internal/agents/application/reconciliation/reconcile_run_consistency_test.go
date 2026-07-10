package reconciliation

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type DecideViolationsSuite struct {
	suite.Suite
}

func TestDecideViolationsSuite(t *testing.T) {
	suite.Run(t, new(DecideViolationsSuite))
}

func (s *DecideViolationsSuite) TestScenarios() {
	scenarios := []struct {
		name   string
		row    RunConsistencyRow
		expect []ViolationKind
	}{
		{
			name: "sucesso persistido com ledger e workflow concordante nao viola",
			row: RunConsistencyRow{
				RunID:            "run-ok",
				CorrelationKey:   "wamid-1",
				RunStatus:        agent.RunStatusSucceeded,
				Outcome:          agent.ToolOutcomeRouted,
				LedgerWrites:     1,
				TransactionCount: 1,
				WorkflowStatus:   workflow.RunStatusSucceeded,
				WorkflowFound:    true,
			},
			expect: nil,
		},
		{
			name: "correlation_key vazio viola",
			row: RunConsistencyRow{
				RunID:          "run-empty",
				CorrelationKey: "",
				RunStatus:      agent.RunStatusSucceeded,
				Outcome:        agent.ToolOutcomeReplay,
			},
			expect: []ViolationKind{ViolationEmptyCorrelationKey},
		},
		{
			name: "sucesso routed sem efeito e sem workflow succeeded viola",
			row: RunConsistencyRow{
				RunID:          "run-noeffect",
				CorrelationKey: "wamid-2",
				RunStatus:      agent.RunStatusSucceeded,
				Outcome:        agent.ToolOutcomeRouted,
				LedgerWrites:   0,
			},
			expect: []ViolationKind{ViolationSucceededNoEffect},
		},
		{
			name: "sucesso routed sem ledger mas workflow succeeded nao viola",
			row: RunConsistencyRow{
				RunID:          "run-wfok",
				CorrelationKey: "wamid-3",
				RunStatus:      agent.RunStatusSucceeded,
				Outcome:        agent.ToolOutcomeRouted,
				LedgerWrites:   0,
				WorkflowStatus: workflow.RunStatusSucceeded,
				WorkflowFound:  true,
			},
			expect: nil,
		},
		{
			name: "failed com write pre-existente viola orfao",
			row: RunConsistencyRow{
				RunID:          "run-orphan",
				CorrelationKey: "wamid-4",
				RunStatus:      agent.RunStatusFailed,
				Outcome:        agent.ToolOutcomeUsecaseError,
				LedgerWrites:   1,
				WorkflowStatus: workflow.RunStatusFailed,
				WorkflowFound:  true,
			},
			expect: []ViolationKind{ViolationFailedOrphanWrite},
		},
		{
			name: "divergencia de status agent vs workflow viola",
			row: RunConsistencyRow{
				RunID:            "run-divergent",
				CorrelationKey:   "wamid-5",
				RunStatus:        agent.RunStatusSucceeded,
				Outcome:          agent.ToolOutcomeRouted,
				LedgerWrites:     1,
				TransactionCount: 1,
				WorkflowStatus:   workflow.RunStatusFailed,
				WorkflowFound:    true,
			},
			expect: []ViolationKind{ViolationStatusDivergence},
		},
		{
			name: "workflow running nao conta como divergencia",
			row: RunConsistencyRow{
				RunID:            "run-running",
				CorrelationKey:   "wamid-6",
				RunStatus:        agent.RunStatusSucceeded,
				Outcome:          agent.ToolOutcomeRouted,
				LedgerWrites:     1,
				TransactionCount: 1,
				WorkflowStatus:   workflow.RunStatusRunning,
				WorkflowFound:    true,
			},
			expect: nil,
		},
		{
			name: "state.status concordante com wr.status nao viola",
			row: RunConsistencyRow{
				RunID:             "run-state-ok",
				CorrelationKey:    "wamid-8",
				RunStatus:         agent.RunStatusSucceeded,
				Outcome:           agent.ToolOutcomeRouted,
				LedgerWrites:      1,
				TransactionCount:  1,
				WorkflowStatus:    workflow.RunStatusSucceeded,
				WorkflowStateSet:  true,
				WorkflowStateText: "succeeded",
				WorkflowFound:     true,
			},
			expect: nil,
		},
		{
			name: "state.status divergente de wr.status viola",
			row: RunConsistencyRow{
				RunID:             "run-state-diverge",
				CorrelationKey:    "wamid-9",
				RunStatus:         agent.RunStatusSucceeded,
				Outcome:           agent.ToolOutcomeRouted,
				LedgerWrites:      1,
				TransactionCount:  1,
				WorkflowStatus:    workflow.RunStatusSucceeded,
				WorkflowStateSet:  true,
				WorkflowStateText: "failed",
				WorkflowFound:     true,
			},
			expect: []ViolationKind{ViolationWorkflowStateDivergence},
		},
		{
			name: "clarify sem efeito nao viola no-effect",
			row: RunConsistencyRow{
				RunID:          "run-clarify",
				CorrelationKey: "wamid-7",
				RunStatus:      agent.RunStatusSucceeded,
				Outcome:        agent.ToolOutcomeClarify,
				LedgerWrites:   0,
			},
			expect: nil,
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			got := DecideViolations([]RunConsistencyRow{scenario.row})
			gotKinds := make([]ViolationKind, 0, len(got))
			for _, v := range got {
				s.Equal(scenario.row.RunID, v.RunID)
				gotKinds = append(gotKinds, v.Kind)
			}
			s.ElementsMatch(scenario.expect, gotKinds)
		})
	}
}
