package workflows

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type fakeGoalEditWorkingMemory struct {
	content       map[string]string
	metadata      map[string]map[string]any
	upsertErr     error
	metadataErr   error
	getErr        error
	upsertCalls   int
	metadataCalls int
}

func newFakeGoalEditWorkingMemory() *fakeGoalEditWorkingMemory {
	return &fakeGoalEditWorkingMemory{
		content:  map[string]string{},
		metadata: map[string]map[string]any{},
	}
}

func (f *fakeGoalEditWorkingMemory) Get(_ context.Context, resourceID string) (string, error) {
	if f.getErr != nil {
		return "", f.getErr
	}
	return f.content[resourceID], nil
}

func (f *fakeGoalEditWorkingMemory) Upsert(_ context.Context, resourceID, content string) error {
	f.upsertCalls++
	if f.upsertErr != nil {
		return f.upsertErr
	}
	f.content[resourceID] = content
	return nil
}

func (f *fakeGoalEditWorkingMemory) UpsertMetadata(_ context.Context, resourceID string, metadata map[string]any) error {
	f.metadataCalls++
	if f.metadataErr != nil {
		return f.metadataErr
	}
	f.metadata[resourceID] = metadata
	return nil
}

type GoalEditWorkflowSuite struct {
	suite.Suite
	ctx        context.Context
	workingMem *fakeGoalEditWorkingMemory
	resourceID string
}

func TestGoalEditWorkflowSuite(t *testing.T) {
	suite.Run(t, new(GoalEditWorkflowSuite))
}

func (s *GoalEditWorkflowSuite) SetupTest() {
	s.ctx = context.Background()
	s.workingMem = newFakeGoalEditWorkingMemory()
	s.resourceID = "user-1"
}

func (s *GoalEditWorkflowSuite) TestBuildGoalEditWorkflow_Definition() {
	def := BuildGoalEditWorkflow(s.workingMem)
	s.Equal(GoalEditWorkflowID, def.ID)
	s.True(def.Durable)
	s.Equal(1, def.MaxAttempts)
	s.NotNil(def.Root)
}

func (s *GoalEditWorkflowSuite) TestFirstEntryReadsCurrentGoalAndSuspends() {
	s.workingMem.content[s.resourceID] = "## Objetivo Financeiro\n\nComprar uma casa"

	def := BuildGoalEditWorkflow(s.workingMem)
	state := GoalEditState{ResourceID: s.resourceID}

	out, err := def.Root.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(workflow.StepStatusSuspended, out.Status)
	s.Equal(GoalEditAwaitingGoal, out.State.Awaiting)
	s.Equal("Comprar uma casa", out.State.PreviousGoal)
	s.Contains(out.Suspend.Prompt, "Comprar uma casa")
}

func (s *GoalEditWorkflowSuite) TestGoalSlotAdvancesToConfirm() {
	def := BuildGoalEditWorkflow(s.workingMem)
	state := GoalEditState{
		ResourceID:   s.resourceID,
		Awaiting:     GoalEditAwaitingGoal,
		PreviousGoal: "Comprar uma casa",
		SuspendedAt:  time.Now().UTC(),
		ResumeText:   "Viajar pela Europa",
	}

	out, err := def.Root.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(workflow.StepStatusSuspended, out.Status)
	s.Equal(GoalEditAwaitingConfirm, out.State.Awaiting)
	s.Equal("Viajar pela Europa", out.State.NewGoal)
	s.Contains(out.Suspend.Prompt, "Viajar pela Europa")
}

func (s *GoalEditWorkflowSuite) TestConfirmExecutesReadModifyWritePreservingOtherSections() {
	s.workingMem.content[s.resourceID] = "## Preferencias\n\nGosta de relatorios\n\n## Objetivo Financeiro\n\nComprar uma casa"

	def := BuildGoalEditWorkflow(s.workingMem)
	state := GoalEditState{
		ResourceID:  s.resourceID,
		Awaiting:    GoalEditAwaitingConfirm,
		NewGoal:     "Viajar pela Europa",
		SuspendedAt: time.Now().UTC(),
		ResumeText:  "sim",
	}

	out, err := def.Root.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(workflow.StepStatusCompleted, out.Status)
	s.Equal(GoalEditCompleted, out.State.Status)
	s.Contains(out.State.ResponseText, "atualizado com sucesso")
	s.Equal(1, s.workingMem.upsertCalls)
	s.Equal(1, s.workingMem.metadataCalls)
	updated := s.workingMem.content[s.resourceID]
	s.Contains(updated, "## Preferencias")
	s.Contains(updated, "Gosta de relatorios")
	s.Contains(updated, "Viajar pela Europa")
	s.NotContains(updated, "Comprar uma casa")
}

func (s *GoalEditWorkflowSuite) TestConfirmCancel() {
	def := BuildGoalEditWorkflow(s.workingMem)
	state := GoalEditState{
		ResourceID:  s.resourceID,
		Awaiting:    GoalEditAwaitingConfirm,
		NewGoal:     "Viajar pela Europa",
		SuspendedAt: time.Now().UTC(),
		ResumeText:  "não",
	}

	out, err := def.Root.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(workflow.StepStatusCompleted, out.Status)
	s.Equal(GoalEditCancelled, out.State.Status)
	s.Equal(0, s.workingMem.upsertCalls)
}

func (s *GoalEditWorkflowSuite) TestBuildGoalEditReaper() {
	reaper := BuildGoalEditReaper(nil, fake.NewProvider())
	s.NotNil(reaper)
}
