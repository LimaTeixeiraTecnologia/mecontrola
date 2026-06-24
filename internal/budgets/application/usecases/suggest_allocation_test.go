package usecases

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
)

type SuggestAllocationSuite struct {
	suite.Suite
	ctx     context.Context
	useCase *SuggestAllocation
}

func TestSuggestAllocationSuite(t *testing.T) {
	suite.Run(t, new(SuggestAllocationSuite))
}

func (s *SuggestAllocationSuite) SetupTest() {
	s.ctx = context.Background()
	s.useCase = NewSuggestAllocation()
}

func (s *SuggestAllocationSuite) TestExecute_SumEqualsTotalCents() {
	type args struct {
		in SuggestAllocationInput
	}
	type dependencies struct{}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(result SuggestAllocationResult, err error)
	}{
		{
			name: "distribuição igual 20% cada soma totalCents",
			args: args{in: SuggestAllocationInput{
				TotalCents: 10000,
				Allocations: []AllocationBP{
					{RootSlug: "expense.custo_fixo", BasisPoints: 2000},
					{RootSlug: "expense.conhecimento", BasisPoints: 2000},
					{RootSlug: "expense.prazeres", BasisPoints: 2000},
					{RootSlug: "expense.metas", BasisPoints: 2000},
					{RootSlug: "expense.liberdade_financeira", BasisPoints: 2000},
				},
			}},
			dependencies: dependencies{},
			expect: func(result SuggestAllocationResult, err error) {
				s.NoError(err)
				s.Len(result.Allocations, 5)
				var sum int64
				for _, a := range result.Allocations {
					sum += a.PlannedCents
				}
				s.Equal(int64(10000), sum)
			},
		},
		{
			name: "residuo positivo — soma ainda igual ao total",
			args: args{in: SuggestAllocationInput{
				TotalCents: 10001,
				Allocations: []AllocationBP{
					{RootSlug: "expense.custo_fixo", BasisPoints: 2000},
					{RootSlug: "expense.conhecimento", BasisPoints: 2000},
					{RootSlug: "expense.prazeres", BasisPoints: 2000},
					{RootSlug: "expense.metas", BasisPoints: 2000},
					{RootSlug: "expense.liberdade_financeira", BasisPoints: 2000},
				},
			}},
			dependencies: dependencies{},
			expect: func(result SuggestAllocationResult, err error) {
				s.NoError(err)
				var sum int64
				for _, a := range result.Allocations {
					sum += a.PlannedCents
				}
				s.Equal(int64(10001), sum)
			},
		},
		{
			name: "arredondamento half-even — soma fecha exato",
			args: args{in: SuggestAllocationInput{
				TotalCents: 3,
				Allocations: []AllocationBP{
					{RootSlug: "expense.custo_fixo", BasisPoints: 5000},
					{RootSlug: "expense.conhecimento", BasisPoints: 5000},
					{RootSlug: "expense.prazeres", BasisPoints: 0},
					{RootSlug: "expense.metas", BasisPoints: 0},
					{RootSlug: "expense.liberdade_financeira", BasisPoints: 0},
				},
			}},
			dependencies: dependencies{},
			expect: func(result SuggestAllocationResult, err error) {
				s.NoError(err)
				var sum int64
				for _, a := range result.Allocations {
					sum += a.PlannedCents
				}
				s.Equal(int64(3), sum)
			},
		},
		{
			name: "basis_points zero na entrada — soma fecha sem erro",
			args: args{in: SuggestAllocationInput{
				TotalCents: 100,
				Allocations: []AllocationBP{
					{RootSlug: "expense.custo_fixo", BasisPoints: 10000},
					{RootSlug: "expense.conhecimento", BasisPoints: 0},
					{RootSlug: "expense.prazeres", BasisPoints: 0},
					{RootSlug: "expense.metas", BasisPoints: 0},
					{RootSlug: "expense.liberdade_financeira", BasisPoints: 0},
				},
			}},
			dependencies: dependencies{},
			expect: func(result SuggestAllocationResult, err error) {
				s.NoError(err)
				var sum int64
				for _, a := range result.Allocations {
					sum += a.PlannedCents
				}
				s.Equal(int64(100), sum)
			},
		},
		{
			name: "basis_points negativo retorna erro de validacao",
			args: args{in: SuggestAllocationInput{
				TotalCents: 10000,
				Allocations: []AllocationBP{
					{RootSlug: "expense.custo_fixo", BasisPoints: -1},
				},
			}},
			dependencies: dependencies{},
			expect: func(result SuggestAllocationResult, err error) {
				s.Error(err)
				s.Empty(result.Allocations)
			},
		},
		{
			name: "total_cents zero retorna erro de validacao",
			args: args{in: SuggestAllocationInput{
				TotalCents: 0,
				Allocations: []AllocationBP{
					{RootSlug: "expense.custo_fixo", BasisPoints: 10000},
				},
			}},
			dependencies: dependencies{},
			expect: func(result SuggestAllocationResult, err error) {
				s.Error(err)
				s.Empty(result.Allocations)
			},
		},
		{
			name: "allocations vazio retorna erro de validacao",
			args: args{in: SuggestAllocationInput{
				TotalCents:  10000,
				Allocations: []AllocationBP{},
			}},
			dependencies: dependencies{},
			expect: func(result SuggestAllocationResult, err error) {
				s.Error(err)
				s.Empty(result.Allocations)
			},
		},
		{
			name: "root_slug invalido retorna erro",
			args: args{in: SuggestAllocationInput{
				TotalCents: 10000,
				Allocations: []AllocationBP{
					{RootSlug: "expense.invalido", BasisPoints: 10000},
				},
			}},
			dependencies: dependencies{},
			expect: func(result SuggestAllocationResult, err error) {
				s.Error(err)
				s.Empty(result.Allocations)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			uc := NewSuggestAllocation()
			result, err := uc.Execute(s.ctx, scenario.args.in)
			scenario.expect(result, err)
		})
	}
}
