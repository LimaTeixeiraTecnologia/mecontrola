package workflow

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

func stubTool(name string, kind intent.Kind, result tools.ToolResult, calls *int) tools.Tool {
	return tools.NewTool(tools.ToolSpec{Name: name, IntentKind: kind, Description: name}, func(_ context.Context, _ tools.ToolInput) (tools.ToolResult, error) {
		if calls != nil {
			*calls++
		}
		return result, nil
	})
}

func writeIntent(s *suite.Suite) intent.Intent {
	in, err := intent.NewRecordExpense(intent.RecordExpenseFields{AmountCents: 5800, Merchant: "iFood", CategoryHint: "Prazeres"})
	require.NoError(s.T(), err)
	return in
}

type CompositeWorkflowSuite struct {
	suite.Suite
	ctx context.Context
}

func TestCompositeWorkflowSuite(t *testing.T) {
	suite.Run(t, new(CompositeWorkflowSuite))
}

func (s *CompositeWorkflowSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *CompositeWorkflowSuite) TestReadKindRunsTool() {
	calls := 0
	wf, err := NewIntentWorkflow("budget",
		KindTool{Kind: intent.KindHowAmIDoing, Tool: stubTool("how", intent.KindHowAmIDoing, tools.ToolResult{Reply: "ok", Outcome: tools.OutcomeRouted, Kind: intent.KindHowAmIDoing}, &calls)},
	)
	s.Require().NoError(err)
	s.True(wf.Handles(intent.KindHowAmIDoing))
	result, execErr := wf.Execute(s.ctx, tools.ToolInput{UserID: uuid.New(), Channel: "whatsapp", Intent: intent.NewHowAmIDoing()})
	s.NoError(execErr)
	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Equal(1, calls)
}

func (s *CompositeWorkflowSuite) TestWriteKindRunsTool() {
	calls := 0
	wf, err := NewIntentWorkflow("transactions",
		KindTool{Kind: intent.KindRecordExpense, Tool: stubTool("expense", intent.KindRecordExpense, tools.ToolResult{Reply: "ok", Outcome: tools.OutcomeRouted, Kind: intent.KindRecordExpense}, &calls)},
	)
	s.Require().NoError(err)
	result, execErr := wf.Execute(s.ctx, tools.ToolInput{UserID: uuid.New(), Channel: "whatsapp", Intent: writeIntent(&s.Suite)})
	s.NoError(execErr)
	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Equal(1, calls)
}

func (s *CompositeWorkflowSuite) TestExecuteKindNotHandled() {
	wf, err := NewIntentWorkflow("cards",
		KindTool{Kind: intent.KindListCards, Tool: stubTool("list", intent.KindListCards, tools.ToolResult{Outcome: tools.OutcomeRouted}, nil)},
	)
	s.Require().NoError(err)
	result, execErr := wf.Execute(s.ctx, tools.ToolInput{UserID: uuid.New(), Channel: "whatsapp", Intent: intent.NewCountCards()})
	s.ErrorIs(execErr, ErrKindNotHandled)
	s.Equal(tools.OutcomeUsecaseError, result.Outcome)
}

func (s *CompositeWorkflowSuite) TestConstructorRejectsEmptyAndNil() {
	_, err := NewIntentWorkflow("", KindTool{Kind: intent.KindListCards, Tool: stubTool("x", intent.KindListCards, tools.ToolResult{}, nil)})
	s.ErrorIs(err, ErrWorkflowIDEmpty)

	_, err = NewIntentWorkflow("cards")
	s.ErrorIs(err, ErrNoTools)

	_, err = NewIntentWorkflow("cards", KindTool{Kind: intent.KindListCards, Tool: nil})
	s.ErrorIs(err, ErrToolForKindNil)
}

type RegistrySuite struct {
	suite.Suite
}

func TestRegistrySuite(t *testing.T) {
	suite.Run(t, new(RegistrySuite))
}

func (s *RegistrySuite) buildWorkflows() (IntentWorkflow, IntentWorkflow) {
	cards, err := NewIntentWorkflow("cards",
		KindTool{Kind: intent.KindListCards, Tool: stubTool("list", intent.KindListCards, tools.ToolResult{Outcome: tools.OutcomeRouted}, nil)},
		KindTool{Kind: intent.KindCountCards, Tool: stubTool("count", intent.KindCountCards, tools.ToolResult{Outcome: tools.OutcomeRouted}, nil)},
	)
	s.Require().NoError(err)
	budget, err := NewIntentWorkflow("budget",
		KindTool{Kind: intent.KindHowAmIDoing, Tool: stubTool("how", intent.KindHowAmIDoing, tools.ToolResult{Outcome: tools.OutcomeRouted}, nil)},
	)
	s.Require().NoError(err)
	return cards, budget
}

func (s *RegistrySuite) TestResolveByKind() {
	cards, budget := s.buildWorkflows()
	registry, err := NewIntentRegistry([]intent.Kind{intent.KindListCards, intent.KindCountCards, intent.KindHowAmIDoing}, cards, budget)
	s.Require().NoError(err)

	wf, ok := registry.Resolve(intent.KindListCards)
	s.True(ok)
	s.Equal("cards", wf.ID())

	wf, ok = registry.Resolve(intent.KindHowAmIDoing)
	s.True(ok)
	s.Equal("budget", wf.ID())

	_, ok = registry.Resolve(intent.KindRecordExpense)
	s.False(ok)
}

func (s *RegistrySuite) TestEmptyRegistry() {
	_, err := NewIntentRegistry(nil)
	s.ErrorIs(err, ErrEmptyRegistry)
}

func (s *RegistrySuite) TestDuplicateID() {
	cards, _ := s.buildWorkflows()
	_, err := NewIntentRegistry([]intent.Kind{intent.KindListCards}, cards, cards)
	s.ErrorIs(err, ErrDuplicateWorkflow)
}
