package tools

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

type stubBudgetRecurrenceCreator struct {
	result BudgetRecurrenceCreatorResult
	err    error
}

func (s *stubBudgetRecurrenceCreator) Execute(_ context.Context, _ BudgetRecurrenceCreatorInput) (BudgetRecurrenceCreatorResult, error) {
	return s.result, s.err
}

type BudgetRecurrenceCreatorToolSuite struct {
	suite.Suite
	ctx context.Context
	loc *time.Location
}

func TestBudgetRecurrenceCreatorToolSuite(t *testing.T) {
	suite.Run(t, new(BudgetRecurrenceCreatorToolSuite))
}

func (s *BudgetRecurrenceCreatorToolSuite) SetupTest() {
	s.ctx = context.Background()
	s.loc = time.UTC
}

func (s *BudgetRecurrenceCreatorToolSuite) newTool(creator BudgetRecurrenceCreator) *BudgetRecurrenceCreatorTool {
	obs := fake.NewProvider()
	counter := obs.Metrics().Counter("test_routed_total", "", "1")
	recorder := NewRecorder(counter)
	return NewBudgetRecurrenceCreatorTool(recorder, creator, s.loc, obs)
}

func (s *BudgetRecurrenceCreatorToolSuite) toolInput(sourceCompetence string, months int) ToolInput {
	i, _ := intent.NewBudgetRecurrence(intent.BudgetRecurrenceFields{
		SourceCompetence: sourceCompetence,
		Months:           months,
	})
	return ToolInput{
		UserID:  uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		Channel: "whatsapp",
		Intent:  i,
	}
}

func (s *BudgetRecurrenceCreatorToolSuite) TestExecute() {
	type args struct {
		sourceCompetence string
		months           int
	}
	type dependencies struct {
		creator BudgetRecurrenceCreator
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(result ToolResult, err error)
	}{
		{
			name: "deve replicar orçamento com sucesso",
			args: args{sourceCompetence: "2026-06", months: 3},
			dependencies: dependencies{
				creator: &stubBudgetRecurrenceCreator{
					result: BudgetRecurrenceCreatorResult{SourceCompetence: "2026-06", MonthsCreated: 3},
				},
			},
			expect: func(result ToolResult, err error) {
				s.NoError(err)
				s.Equal(OutcomeRouted, result.Outcome)
				s.Contains(result.Reply, "2026-06")
				s.Contains(result.Reply, "3 meses")
			},
		},
		{
			name: "deve replicar orçamento por 1 mês com mensagem singular",
			args: args{sourceCompetence: "2026-06", months: 1},
			dependencies: dependencies{
				creator: &stubBudgetRecurrenceCreator{
					result: BudgetRecurrenceCreatorResult{SourceCompetence: "2026-06", MonthsCreated: 1},
				},
			},
			expect: func(result ToolResult, err error) {
				s.NoError(err)
				s.Equal(OutcomeRouted, result.Outcome)
				s.Contains(result.Reply, "próximo mês")
			},
		},
		{
			name: "deve retornar erro de usecase",
			args: args{sourceCompetence: "2026-06", months: 2},
			dependencies: dependencies{
				creator: &stubBudgetRecurrenceCreator{
					err: errors.New("falha no banco"),
				},
			},
			expect: func(result ToolResult, err error) {
				s.NoError(err)
				s.Equal(OutcomeUsecaseError, result.Outcome)
				s.Equal(fallbackUsecaseError, result.Reply)
			},
		},
		{
			name:         "deve retornar missing resolver quando creator é nil",
			args:         args{sourceCompetence: "2026-06", months: 1},
			dependencies: dependencies{creator: nil},
			expect: func(result ToolResult, err error) {
				s.NoError(err)
				s.Equal(OutcomeMissingResolver, result.Outcome)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			tool := s.newTool(scenario.dependencies.creator)
			in := s.toolInput(scenario.args.sourceCompetence, scenario.args.months)
			result, err := tool.Execute(s.ctx, in)
			scenario.expect(result, err)
		})
	}
}
