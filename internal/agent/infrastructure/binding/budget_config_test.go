package binding

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/budgetdraft"
	budgetsinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	budgetsoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	budgetsinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	budgetsentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
)

type fakeCreateBudgetUC struct {
	out budgetsoutput.BudgetOutput
	err error
}

func (f *fakeCreateBudgetUC) Execute(_ context.Context, _ budgetsinput.CreateBudgetInput) (budgetsoutput.BudgetOutput, error) {
	return f.out, f.err
}

type fakeActivateBudgetUC struct {
	out budgetsoutput.BudgetOutput
	err error
}

func (f *fakeActivateBudgetUC) Execute(_ context.Context, _ budgetsinput.ActivateBudgetInput) (budgetsoutput.BudgetOutput, error) {
	return f.out, f.err
}

type BudgetConfigSuite struct {
	suite.Suite
	ctx    context.Context
	userID uuid.UUID
}

func TestBudgetConfigSuite(t *testing.T) {
	suite.Run(t, new(BudgetConfigSuite))
}

func (s *BudgetConfigSuite) SetupTest() {
	s.ctx = context.Background()
	s.userID = uuid.New()
}

func (s *BudgetConfigSuite) buildDraft() budgetdraft.Draft {
	d := budgetdraft.New("2026-06")
	d, err := d.Merge(budgetdraft.Change{
		TotalCents: 500000,
		Allocations: map[string]int{
			budgetdraft.SlugCustoFixo:           5000,
			budgetdraft.SlugConhecimento:        1000,
			budgetdraft.SlugPrazeres:            1000,
			budgetdraft.SlugMetas:               1000,
			budgetdraft.SlugLiberdadeFinanceira: 2000,
		},
	})
	s.Require().NoError(err)
	return d
}

func (s *BudgetConfigSuite) TestCommit_Success() {
	createUC := &fakeCreateBudgetUC{out: budgetsoutput.BudgetOutput{Competence: "2026-06", TotalCents: 500000}}
	activateUC := &fakeActivateBudgetUC{out: budgetsoutput.BudgetOutput{Competence: "2026-06", TotalCents: 500000}}
	adapter := NewBudgetConfigCommitterAdapter(createUC, activateUC)

	reply, err := adapter.Commit(s.ctx, s.userID, s.buildDraft())

	s.Require().NoError(err)
	s.Contains(reply, "2026-06")
	s.Contains(reply, "R$ 5000,00")
}

func (s *BudgetConfigSuite) TestCommit_CreateReturnsErrBudgetConflict() {
	createUC := &fakeCreateBudgetUC{err: budgetsinterfaces.ErrBudgetConflict}
	activateUC := &fakeActivateBudgetUC{}
	adapter := NewBudgetConfigCommitterAdapter(createUC, activateUC)

	reply, err := adapter.Commit(s.ctx, s.userID, s.buildDraft())

	s.Require().Error(err)
	s.True(errors.Is(err, budgetsinterfaces.ErrBudgetConflict))
	s.Contains(reply, "substituir")
}

func (s *BudgetConfigSuite) TestCommit_ActivateReturnsErrAllocationSum() {
	createUC := &fakeCreateBudgetUC{out: budgetsoutput.BudgetOutput{Competence: "2026-06"}}
	activateUC := &fakeActivateBudgetUC{err: budgetsentities.ErrBudgetAllocationSumMustBe10000}
	adapter := NewBudgetConfigCommitterAdapter(createUC, activateUC)

	reply, err := adapter.Commit(s.ctx, s.userID, s.buildDraft())

	s.Require().Error(err)
	s.True(errors.Is(err, budgetsentities.ErrBudgetAllocationSumMustBe10000))
	s.Contains(reply, "100%")
}

func (s *BudgetConfigSuite) TestCommit_ActivateReturnsErrBudgetConflict() {
	createUC := &fakeCreateBudgetUC{out: budgetsoutput.BudgetOutput{Competence: "2026-06"}}
	activateUC := &fakeActivateBudgetUC{err: budgetsinterfaces.ErrBudgetConflict}
	adapter := NewBudgetConfigCommitterAdapter(createUC, activateUC)

	reply, err := adapter.Commit(s.ctx, s.userID, s.buildDraft())

	s.Require().Error(err)
	s.True(errors.Is(err, budgetsinterfaces.ErrBudgetConflict))
	s.Contains(reply, "substituir")
}

func (s *BudgetConfigSuite) TestCommit_ActivateGenericError() {
	createUC := &fakeCreateBudgetUC{out: budgetsoutput.BudgetOutput{Competence: "2026-06"}}
	activateUC := &fakeActivateBudgetUC{err: errors.New("infra failure")}
	adapter := NewBudgetConfigCommitterAdapter(createUC, activateUC)

	reply, err := adapter.Commit(s.ctx, s.userID, s.buildDraft())

	s.Require().Error(err)
	s.Contains(reply, "rascunho")
}

func (s *BudgetConfigSuite) TestCommit_EmptyCompetenceFallsBackToNow() {
	draft := budgetdraft.New("")
	draft, err := draft.Merge(budgetdraft.Change{
		TotalCents: 100000,
		Allocations: map[string]int{
			budgetdraft.SlugCustoFixo:           5000,
			budgetdraft.SlugConhecimento:        1000,
			budgetdraft.SlugPrazeres:            1000,
			budgetdraft.SlugMetas:               1000,
			budgetdraft.SlugLiberdadeFinanceira: 2000,
		},
	})
	s.Require().NoError(err)

	createUC := &fakeCreateBudgetUC{out: budgetsoutput.BudgetOutput{Competence: "2026-06", TotalCents: 100000}}
	activateUC := &fakeActivateBudgetUC{out: budgetsoutput.BudgetOutput{Competence: "2026-06", TotalCents: 100000}}
	adapter := NewBudgetConfigCommitterAdapter(createUC, activateUC)

	reply, commitErr := adapter.Commit(s.ctx, s.userID, draft)

	s.Require().NoError(commitErr)
	s.NotEmpty(reply)
}
