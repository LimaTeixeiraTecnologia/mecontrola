package services_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type CategoryPercentageWorkflowSuite struct {
	suite.Suite
}

func TestCategoryPercentageWorkflowSuite(t *testing.T) {
	suite.Run(t, new(CategoryPercentageWorkflowSuite))
}

func mustSlug(s string) valueobjects.RootSlug {
	slug, err := valueobjects.ParseRootSlug(s)
	if err != nil {
		panic(err)
	}
	return slug
}

func mustBP(v int) valueobjects.BasisPoints {
	bp, err := valueobjects.NewBasisPoints(v)
	if err != nil {
		panic(err)
	}
	return bp
}

func sumBasisPoints(in []services.CategoryPercentageInput) int {
	total := 0
	for _, a := range in {
		total += a.BasisPoints
	}
	return total
}

func findBasisPoints(in []services.CategoryPercentageInput, slug valueobjects.RootSlug) int {
	for _, a := range in {
		if a.RootSlug == slug {
			return a.BasisPoints
		}
	}
	return -1
}

func (s *CategoryPercentageWorkflowSuite) TestProportionalRedistribution() {
	current := []services.CategoryPercentageInput{
		{RootSlug: mustSlug("expense.custo_fixo"), BasisPoints: 5000},
		{RootSlug: mustSlug("expense.conhecimento"), BasisPoints: 3000},
		{RootSlug: mustSlug("expense.prazeres"), BasisPoints: 2000},
	}

	result, err := services.DecideEditCategoryPercentage(current, mustSlug("expense.custo_fixo"), mustBP(6000))

	s.NoError(err)
	s.Equal(10000, sumBasisPoints(result))
	s.Equal(6000, findBasisPoints(result, mustSlug("expense.custo_fixo")))
	s.Equal(2400, findBasisPoints(result, mustSlug("expense.conhecimento")))
	s.Equal(1600, findBasisPoints(result, mustSlug("expense.prazeres")))
}

func (s *CategoryPercentageWorkflowSuite) TestTargetZeroPercent() {
	current := []services.CategoryPercentageInput{
		{RootSlug: mustSlug("expense.custo_fixo"), BasisPoints: 4000},
		{RootSlug: mustSlug("expense.conhecimento"), BasisPoints: 3000},
		{RootSlug: mustSlug("expense.prazeres"), BasisPoints: 3000},
	}

	result, err := services.DecideEditCategoryPercentage(current, mustSlug("expense.custo_fixo"), mustBP(0))

	s.NoError(err)
	s.Equal(10000, sumBasisPoints(result))
	s.Equal(0, findBasisPoints(result, mustSlug("expense.custo_fixo")))
	s.Equal(5000, findBasisPoints(result, mustSlug("expense.conhecimento")))
	s.Equal(5000, findBasisPoints(result, mustSlug("expense.prazeres")))
}

func (s *CategoryPercentageWorkflowSuite) TestTargetHundredPercent() {
	current := []services.CategoryPercentageInput{
		{RootSlug: mustSlug("expense.custo_fixo"), BasisPoints: 4000},
		{RootSlug: mustSlug("expense.conhecimento"), BasisPoints: 3000},
		{RootSlug: mustSlug("expense.prazeres"), BasisPoints: 3000},
	}

	result, err := services.DecideEditCategoryPercentage(current, mustSlug("expense.custo_fixo"), mustBP(10000))

	s.NoError(err)
	s.Equal(10000, sumBasisPoints(result))
	s.Equal(10000, findBasisPoints(result, mustSlug("expense.custo_fixo")))
	s.Equal(0, findBasisPoints(result, mustSlug("expense.conhecimento")))
	s.Equal(0, findBasisPoints(result, mustSlug("expense.prazeres")))
}

func (s *CategoryPercentageWorkflowSuite) TestRoundingLeftoverToLargestOther() {
	current := []services.CategoryPercentageInput{
		{RootSlug: mustSlug("expense.custo_fixo"), BasisPoints: 5000},
		{RootSlug: mustSlug("expense.conhecimento"), BasisPoints: 3333},
		{RootSlug: mustSlug("expense.prazeres"), BasisPoints: 1667},
	}

	result, err := services.DecideEditCategoryPercentage(current, mustSlug("expense.custo_fixo"), mustBP(3333))

	s.NoError(err)
	s.Equal(10000, sumBasisPoints(result))
	s.Equal(3333, findBasisPoints(result, mustSlug("expense.custo_fixo")))
	conhecimento := findBasisPoints(result, mustSlug("expense.conhecimento"))
	prazeres := findBasisPoints(result, mustSlug("expense.prazeres"))
	s.Equal(6667, conhecimento+prazeres)
	s.GreaterOrEqual(conhecimento, prazeres)
}

func (s *CategoryPercentageWorkflowSuite) TestTargetNotFound() {
	current := []services.CategoryPercentageInput{
		{RootSlug: mustSlug("expense.custo_fixo"), BasisPoints: 6000},
		{RootSlug: mustSlug("expense.conhecimento"), BasisPoints: 4000},
	}

	_, err := services.DecideEditCategoryPercentage(current, mustSlug("expense.metas"), mustBP(5000))

	s.ErrorIs(err, services.ErrCategoryPercentageTargetNotFound)
}

func (s *CategoryPercentageWorkflowSuite) TestEmptyAllocations() {
	_, err := services.DecideEditCategoryPercentage(nil, mustSlug("expense.custo_fixo"), mustBP(5000))

	s.ErrorIs(err, services.ErrCategoryPercentageNoAllocations)
}

func (s *CategoryPercentageWorkflowSuite) TestCurrentSumInvalid() {
	current := []services.CategoryPercentageInput{
		{RootSlug: mustSlug("expense.custo_fixo"), BasisPoints: 5000},
		{RootSlug: mustSlug("expense.conhecimento"), BasisPoints: 3000},
	}

	_, err := services.DecideEditCategoryPercentage(current, mustSlug("expense.custo_fixo"), mustBP(6000))

	s.ErrorIs(err, services.ErrCategoryPercentageSumInvalid)
}

func (s *CategoryPercentageWorkflowSuite) TestSingleAllocationIsTargetLeftoverGoesToTarget() {
	current := []services.CategoryPercentageInput{
		{RootSlug: mustSlug("expense.custo_fixo"), BasisPoints: 10000},
	}

	result, err := services.DecideEditCategoryPercentage(current, mustSlug("expense.custo_fixo"), mustBP(4000))

	s.NoError(err)
	s.Len(result, 1)
	s.Equal(10000, sumBasisPoints(result))
	s.Equal(10000, findBasisPoints(result, mustSlug("expense.custo_fixo")))
}

func (s *CategoryPercentageWorkflowSuite) TestManyCategoriesSumClosesAt10000() {
	current := []services.CategoryPercentageInput{
		{RootSlug: mustSlug("expense.custo_fixo"), BasisPoints: 2000},
		{RootSlug: mustSlug("expense.conhecimento"), BasisPoints: 2000},
		{RootSlug: mustSlug("expense.prazeres"), BasisPoints: 2000},
		{RootSlug: mustSlug("expense.metas"), BasisPoints: 2000},
		{RootSlug: mustSlug("expense.liberdade_financeira"), BasisPoints: 2000},
	}

	result, err := services.DecideEditCategoryPercentage(current, mustSlug("expense.metas"), mustBP(3333))

	s.NoError(err)
	s.Len(result, 5)
	s.Equal(10000, sumBasisPoints(result))
	s.Equal(3333, findBasisPoints(result, mustSlug("expense.metas")))
}

func (s *CategoryPercentageWorkflowSuite) TestSingleOtherAllocationGetsAll() {
	current := []services.CategoryPercentageInput{
		{RootSlug: mustSlug("expense.custo_fixo"), BasisPoints: 7000},
		{RootSlug: mustSlug("expense.conhecimento"), BasisPoints: 3000},
	}

	result, err := services.DecideEditCategoryPercentage(current, mustSlug("expense.custo_fixo"), mustBP(4000))

	s.NoError(err)
	s.Equal(10000, sumBasisPoints(result))
	s.Equal(4000, findBasisPoints(result, mustSlug("expense.custo_fixo")))
	s.Equal(6000, findBasisPoints(result, mustSlug("expense.conhecimento")))
}
