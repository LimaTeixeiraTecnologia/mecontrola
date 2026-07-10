package golden

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type RatioSuite struct {
	suite.Suite
}

func TestRatioSuite(t *testing.T) {
	suite.Run(t, new(RatioSuite))
}

func (s *RatioSuite) TestCategoryResultRatio() {
	type args struct {
		result CategoryResult
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(ratio float64)
	}{
		{
			name: "deve calcular ratio 1.0 quando todos os casos passam",
			args: args{result: CategoryResult{Hits: 10, Total: 10}},
			expect: func(ratio float64) {
				s.Equal(1.0, ratio)
			},
		},
		{
			name: "deve calcular ratio parcial",
			args: args{result: CategoryResult{Hits: 9, Total: 10}},
			expect: func(ratio float64) {
				s.InDelta(0.9, ratio, 0.0001)
			},
		},
		{
			name: "deve retornar 1.0 trivial quando total e zero",
			args: args{result: CategoryResult{Hits: 0, Total: 0}},
			expect: func(ratio float64) {
				s.Equal(1.0, ratio)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			ratio := scenario.args.result.Ratio()
			scenario.expect(ratio)
		})
	}
}

func (s *RatioSuite) TestCategoryResultPassesGate() {
	type args struct {
		result    CategoryResult
		threshold float64
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(passed bool)
	}{
		{
			name: "deve passar quando ratio igual ao threshold",
			args: args{result: CategoryResult{Hits: 9, Total: 10}, threshold: 0.9},
			expect: func(passed bool) {
				s.True(passed)
			},
		},
		{
			name: "deve falhar quando ratio abaixo do threshold",
			args: args{result: CategoryResult{Hits: 8, Total: 10}, threshold: 0.9},
			expect: func(passed bool) {
				s.False(passed)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			passed := scenario.args.result.PassesGate(scenario.args.threshold)
			scenario.expect(passed)
		})
	}
}

func (s *RatioSuite) TestAggregateByCategory() {
	outcomes := []CaseOutcome{
		{Case: Case{Name: "a", Category: CategoryQuery}, Passed: true},
		{Case: Case{Name: "b", Category: CategoryQuery}, Passed: false, Detail: "tool errada"},
		{Case: Case{Name: "c", Category: CategoryCard}, Passed: true},
	}

	results := AggregateByCategory(outcomes)

	s.Len(results, 2)

	byCategory := make(map[Category]CategoryResult, len(results))
	for _, r := range results {
		byCategory[r.Category] = r
	}

	queryResult := byCategory[CategoryQuery]
	s.Equal(1, queryResult.Hits)
	s.Equal(2, queryResult.Total)
	s.Equal([]string{"b: tool errada"}, queryResult.Failures)

	cardResult := byCategory[CategoryCard]
	s.Equal(1, cardResult.Hits)
	s.Equal(1, cardResult.Total)
	s.Empty(cardResult.Failures)
}

func (s *RatioSuite) TestAggregateByCategoryDeterministicOrder() {
	outcomes := []CaseOutcome{
		{Case: Case{Name: "a", Category: CategoryBudget}, Passed: true},
		{Case: Case{Name: "b", Category: CategoryQuery}, Passed: true},
		{Case: Case{Name: "c", Category: CategoryBudget}, Passed: true},
	}

	first := AggregateByCategory(outcomes)
	second := AggregateByCategory(outcomes)

	s.Equal(first, second)
	s.Equal(CategoryBudget, first[0].Category)
	s.Equal(CategoryQuery, first[1].Category)
}
