package commands_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/commands"
)

type CreateBudgetCommandSuite struct {
	suite.Suite
}

func TestCreateBudgetCommandSuite(t *testing.T) {
	suite.Run(t, new(CreateBudgetCommandSuite))
}

func (s *CreateBudgetCommandSuite) TestNewCreateBudgetCommand() {
	validUserID := "11111111-1111-4111-8111-111111111111"
	good := []commands.AllocationCommandInput{
		{RootSlug: "expense.custo_fixo", BasisPoints: 5000},
		{RootSlug: "expense.prazeres", BasisPoints: 5000},
	}

	cases := []struct {
		name        string
		userID      string
		competence  string
		totalCents  int64
		allocations []commands.AllocationCommandInput
		wantErrs    []error
	}{
		{name: "success", userID: validUserID, competence: "2026-06", totalCents: 10000, allocations: good},
		{name: "invalid_user_id", userID: "x", competence: "2026-06", totalCents: 10000, allocations: good, wantErrs: []error{commands.ErrCommandInvalidUserID}},
		{name: "invalid_competence", userID: validUserID, competence: "bad", totalCents: 10000, allocations: good, wantErrs: []error{commands.ErrCommandInvalidCompetence}},
		{name: "invalid_total", userID: validUserID, competence: "2026-06", totalCents: 0, allocations: good, wantErrs: []error{commands.ErrCommandInvalidTotalCents}},
		{
			name:       "invalid_allocation_slug",
			userID:     validUserID,
			competence: "2026-06",
			totalCents: 10000,
			allocations: []commands.AllocationCommandInput{
				{RootSlug: "unknown.slug", BasisPoints: 5000},
			},
			wantErrs: []error{commands.ErrCommandInvalidAllocation},
		},
		{
			name:       "invalid_allocation_bp_out_of_range",
			userID:     validUserID,
			competence: "2026-06",
			totalCents: 10000,
			allocations: []commands.AllocationCommandInput{
				{RootSlug: "expense.custo_fixo", BasisPoints: -1},
			},
			wantErrs: []error{commands.ErrCommandInvalidAllocation},
		},
		{
			name:       "invalid_allocation_sum_exceeds",
			userID:     validUserID,
			competence: "2026-06",
			totalCents: 10000,
			allocations: []commands.AllocationCommandInput{
				{RootSlug: "expense.custo_fixo", BasisPoints: 7000},
				{RootSlug: "expense.prazeres", BasisPoints: 4000},
			},
			wantErrs: []error{commands.ErrCommandInvalidAllocation},
		},
		{
			name:       "invalid_allocation_sum_11000",
			userID:     validUserID,
			competence: "2026-06",
			totalCents: 10000,
			allocations: []commands.AllocationCommandInput{
				{RootSlug: "expense.custo_fixo", BasisPoints: 6000},
				{RootSlug: "expense.prazeres", BasisPoints: 5000},
			},
			wantErrs: []error{commands.ErrCommandInvalidAllocation},
		},
		{
			name:       "invalid_allocation_sum_9000_partial",
			userID:     validUserID,
			competence: "2026-06",
			totalCents: 10000,
			allocations: []commands.AllocationCommandInput{
				{RootSlug: "expense.custo_fixo", BasisPoints: 5000},
				{RootSlug: "expense.prazeres", BasisPoints: 4000},
			},
			wantErrs: []error{commands.ErrCommandInvalidAllocation},
		},
		{
			name:       "valid_personalizacao_caso_real_5000_1000_4000",
			userID:     validUserID,
			competence: "2026-06",
			totalCents: 500000,
			allocations: []commands.AllocationCommandInput{
				{RootSlug: "expense.custo_fixo", BasisPoints: 5000},
				{RootSlug: "expense.conhecimento", BasisPoints: 0},
				{RootSlug: "expense.prazeres", BasisPoints: 1000},
				{RootSlug: "expense.metas", BasisPoints: 0},
				{RootSlug: "expense.liberdade_financeira", BasisPoints: 4000},
			},
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			cmd, err := commands.NewCreateBudgetCommand(tc.userID, tc.competence, tc.totalCents, tc.allocations)
			if len(tc.wantErrs) == 0 {
				s.Require().NoError(err)
				s.Equal(tc.totalCents, cmd.TotalCents)
				s.Len(cmd.Allocations, len(tc.allocations))
				return
			}
			s.Require().Error(err)
			for _, w := range tc.wantErrs {
				s.True(errors.Is(err, w))
			}
		})
	}
}
