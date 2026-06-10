package services_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type AllocationDistributorSuite struct {
	suite.Suite
}

func TestAllocationDistributorSuite(t *testing.T) {
	suite.Run(t, new(AllocationDistributorSuite))
}

func allInputs(bps [5]int) []services.AllocationInput {
	order := valueobjects.CanonicalOrder()
	inputs := make([]services.AllocationInput, 5)
	for i, slug := range order {
		inputs[i] = services.AllocationInput{RootSlug: slug, BasisPoints: bps[i]}
	}
	return inputs
}

func sumPlanned(results []services.AllocationResult) int64 {
	var total int64
	for _, r := range results {
		total += r.PlannedCents
	}
	return total
}

func (s *AllocationDistributorSuite) TestDistribute() {
	type testCase struct {
		name      string
		total     int64
		bps       [5]int
		wantTotal int64
		wantFirst int64
	}

	cases := []testCase{
		{
			name:      "distribuição igual 20% cada",
			total:     10000,
			bps:       [5]int{2000, 2000, 2000, 2000, 2000},
			wantTotal: 10000,
			wantFirst: 2000,
		},
		{
			name:      "centavo residual positivo — custo_fixo recebe extra",
			total:     10001,
			bps:       [5]int{2000, 2000, 2000, 2000, 2000},
			wantTotal: 10001,
			wantFirst: 2001,
		},
		{
			name:      "distribuição assimétrica sem resíduo",
			total:     200,
			bps:       [5]int{5000, 2500, 1000, 1000, 500},
			wantTotal: 200,
			wantFirst: 100,
		},
		{
			name:      "centavo residual negativo — subtrai custo_fixo",
			total:     9999,
			bps:       [5]int{2000, 2000, 2000, 2000, 2000},
			wantTotal: 9999,
			wantFirst: 1999,
		},
		{
			name:      "total 1 centavo 100% custo_fixo",
			total:     1,
			bps:       [5]int{10000, 0, 0, 0, 0},
			wantTotal: 1,
			wantFirst: 1,
		},
		{
			name:      "half-even com valor divisível",
			total:     100,
			bps:       [5]int{3334, 1666, 2000, 2000, 1000},
			wantTotal: 100,
		},
		{
			name:      "half-even exato — arredonda para par",
			total:     3,
			bps:       [5]int{5000, 5000, 0, 0, 0},
			wantTotal: 3,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			results := services.Distribute(tc.total, allInputs(tc.bps))
			s.Equal(tc.wantTotal, sumPlanned(results), "soma deve ser igual ao total")
			if tc.wantFirst > 0 {
				s.Equal(tc.wantFirst, results[0].PlannedCents, "primeiro elemento (custo_fixo)")
			}
		})
	}
}

func (s *AllocationDistributorSuite) TestOrdemDeterministicaParaResidual() {
	inputs := allInputs([5]int{2000, 2000, 2000, 2000, 2000})
	r1 := services.Distribute(10003, inputs)
	r2 := services.Distribute(10003, inputs)
	for i := range r1 {
		s.Equal(r1[i].PlannedCents, r2[i].PlannedCents, "distribuição deve ser determinística")
	}

	sum := sumPlanned(r1)
	s.Equal(int64(10003), sum)

	s.Equal(int64(2000), r1[0].PlannedCents, "custo_fixo reduzido pelo diff negativo")
	s.Equal(int64(2000), r1[1].PlannedCents, "conhecimento reduzido pelo diff negativo")
	s.Equal(int64(2001), r1[2].PlannedCents, "prazeres mantém arredondamento half-even")
	s.Equal(int64(2001), r1[3].PlannedCents, "metas mantém arredondamento half-even")
	s.Equal(int64(2001), r1[4].PlannedCents, "liberdade_financeira mantém arredondamento half-even")
}

func (s *AllocationDistributorSuite) TestResidualNegativo() {
	inputs := allInputs([5]int{2000, 2000, 2000, 2000, 2000})
	results := services.Distribute(9997, inputs)
	sum := sumPlanned(results)
	s.Equal(int64(9997), sum)

	s.Equal(int64(2000), results[0].PlannedCents, "custo_fixo recebe centavo extra do diff positivo")
	s.Equal(int64(2000), results[1].PlannedCents, "conhecimento recebe centavo extra do diff positivo")
	s.Equal(int64(1999), results[2].PlannedCents, "prazeres sem centavo extra")
	s.Equal(int64(1999), results[3].PlannedCents, "metas sem centavo extra")
	s.Equal(int64(1999), results[4].PlannedCents, "liberdade_financeira sem centavo extra")
}
