package workflows

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type DecidePostWriteSuite struct {
	suite.Suite
}

func TestDecidePostWriteSuite(t *testing.T) {
	suite.Run(t, new(DecidePostWriteSuite))
}

func (s *DecidePostWriteSuite) TestDecidePostWrite() {
	validResource := uuid.New()

	scenarios := []struct {
		name           string
		outcome        agent.ToolOutcome
		resourceID     uuid.UUID
		expectPending  PendingStatus
		expectStep     workflow.StepStatus
		expectErrIs    error
		expectNilError bool
	}{
		{
			name:           "accept com recurso valido completa",
			outcome:        agent.ToolOutcomeRouted,
			resourceID:     validResource,
			expectPending:  PendingStatusCompleted,
			expectStep:     workflow.StepStatusCompleted,
			expectNilError: true,
		},
		{
			name:           "replay com recurso vazio completa sem falha",
			outcome:        agent.ToolOutcomeReplay,
			resourceID:     uuid.Nil,
			expectPending:  PendingStatusCompleted,
			expectStep:     workflow.StepStatusCompleted,
			expectNilError: true,
		},
		{
			name:           "reconciled com recurso valido completa",
			outcome:        agent.ToolOutcomeReconciled,
			resourceID:     validResource,
			expectPending:  PendingStatusCompleted,
			expectStep:     workflow.StepStatusCompleted,
			expectNilError: true,
		},
		{
			name:          "sem replay e recurso vazio falha tipada mantendo active",
			outcome:       agent.ToolOutcomeRouted,
			resourceID:    uuid.Nil,
			expectPending: PendingStatusActive,
			expectStep:    workflow.StepStatusFailed,
			expectErrIs:   ErrWriteAcceptedWithoutResource,
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			pending, step, err := DecidePostWrite(scenario.outcome, scenario.resourceID)
			s.Equal(scenario.expectPending, pending)
			s.Equal(scenario.expectStep, step)
			if scenario.expectNilError {
				s.NoError(err)
			} else {
				s.ErrorIs(err, scenario.expectErrIs)
			}
		})
	}
}
