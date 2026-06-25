package binding

import (
	"context"
	"errors"
	"testing"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	budgetsinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	budgetsoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
)

type fakeCreateRecurrenceUC struct {
	out   budgetsoutput.RecurrenceResultOutput
	err   error
	gotIn budgetsinput.CreateRecurrenceInput
	calls int
}

func (f *fakeCreateRecurrenceUC) Execute(_ context.Context, in budgetsinput.CreateRecurrenceInput) (budgetsoutput.RecurrenceResultOutput, error) {
	f.calls++
	f.gotIn = in
	return f.out, f.err
}

type BudgetRecurrenceBindingSuite struct {
	suite.Suite
	ctx    context.Context
	userID uuid.UUID
}

func TestBudgetRecurrenceBindingSuite(t *testing.T) {
	suite.Run(t, new(BudgetRecurrenceBindingSuite))
}

func (s *BudgetRecurrenceBindingSuite) SetupTest() {
	s.ctx = context.Background()
	s.userID = uuid.New()
}

func (s *BudgetRecurrenceBindingSuite) TestExecuteSuccess() {
	uc := &fakeCreateRecurrenceUC{
		out: budgetsoutput.RecurrenceResultOutput{
			SourceCompetence: "2026-06",
			Results: []budgetsoutput.RecurrenceResultEntry{
				{Competence: "2026-07", Status: budgetsoutput.RecurrenceStatusCreated},
				{Competence: "2026-08", Status: budgetsoutput.RecurrenceStatusCreated},
				{Competence: "2026-09", Status: budgetsoutput.RecurrenceStatusUpdated},
			},
		},
	}
	adapter := NewBudgetRecurrenceCreatorAdapter(uc)

	result, err := adapter.Execute(s.ctx, tools.BudgetRecurrenceCreatorInput{
		UserID:           s.userID,
		SourceCompetence: "2026-06",
		Months:           3,
	})

	s.Require().NoError(err)
	s.Equal(1, uc.calls)
	s.Equal(s.userID.String(), uc.gotIn.UserID)
	s.Equal("2026-06", uc.gotIn.SourceCompetence)
	s.Equal(3, uc.gotIn.Months)
	s.Equal("2026-06", result.SourceCompetence)
	s.Equal(3, result.MonthsCreated)
}

func (s *BudgetRecurrenceBindingSuite) TestExecutePropagatesError() {
	uc := &fakeCreateRecurrenceUC{err: errors.New("falha no banco")}
	adapter := NewBudgetRecurrenceCreatorAdapter(uc)

	_, err := adapter.Execute(s.ctx, tools.BudgetRecurrenceCreatorInput{
		UserID:           s.userID,
		SourceCompetence: "2026-06",
		Months:           1,
	})

	s.Error(err)
}

func (s *BudgetRecurrenceBindingSuite) TestCountCreatedSkipsConflictAndFailure() {
	uc := &fakeCreateRecurrenceUC{
		out: budgetsoutput.RecurrenceResultOutput{
			SourceCompetence: "2026-06",
			Results: []budgetsoutput.RecurrenceResultEntry{
				{Competence: "2026-07", Status: budgetsoutput.RecurrenceStatusCreated},
				{Competence: "2026-08", Status: budgetsoutput.RecurrenceStatusConflict},
				{Competence: "2026-09", Status: budgetsoutput.RecurrenceStatusFailure},
				{Competence: "2026-10", Status: budgetsoutput.RecurrenceStatusCompletedFromDraft},
			},
		},
	}
	adapter := NewBudgetRecurrenceCreatorAdapter(uc)

	result, err := adapter.Execute(s.ctx, tools.BudgetRecurrenceCreatorInput{
		UserID:           s.userID,
		SourceCompetence: "2026-06",
		Months:           4,
	})

	s.Require().NoError(err)
	s.Equal(2, result.MonthsCreated)
}
