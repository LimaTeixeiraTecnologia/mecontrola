package services

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type ApplyLeftoverWhiteboxSuite struct {
	suite.Suite
}

func TestApplyLeftoverWhiteboxSuite(t *testing.T) {
	suite.Run(t, new(ApplyLeftoverWhiteboxSuite))
}

func (s *ApplyLeftoverWhiteboxSuite) slug(v string) valueobjects.RootSlug {
	slug, err := valueobjects.ParseRootSlug(v)
	s.Require().NoError(err)
	return slug
}

func (s *ApplyLeftoverWhiteboxSuite) sum(in []CategoryPercentageInput) int {
	total := 0
	for _, a := range in {
		total += a.BasisPoints
	}
	return total
}

func (s *ApplyLeftoverWhiteboxSuite) TestDeficitDrainsFromLargestOther() {
	target := s.slug("expense.custo_fixo")
	result := []CategoryPercentageInput{
		{RootSlug: target, BasisPoints: 4000},
		{RootSlug: s.slug("expense.conhecimento"), BasisPoints: 4000},
		{RootSlug: s.slug("expense.prazeres"), BasisPoints: 2003},
	}

	applyLeftover(result, target, -3)

	s.Equal(10000, s.sum(result))
	s.Equal(4000, result[0].BasisPoints)
}

func (s *ApplyLeftoverWhiteboxSuite) TestDeficitFallsBackToTargetWhenOthersExhausted() {
	target := s.slug("expense.custo_fixo")
	result := []CategoryPercentageInput{
		{RootSlug: target, BasisPoints: 10005},
		{RootSlug: s.slug("expense.conhecimento"), BasisPoints: 0},
	}

	applyLeftover(result, target, -5)

	s.Equal(10000, s.sum(result))
	s.Equal(10000, result[0].BasisPoints)
	s.Equal(0, result[1].BasisPoints)
}

func (s *ApplyLeftoverWhiteboxSuite) TestPositiveLeftoverFallsBackToTargetWhenNoOthers() {
	target := s.slug("expense.custo_fixo")
	result := []CategoryPercentageInput{
		{RootSlug: target, BasisPoints: 9990},
	}

	applyLeftover(result, target, 10)

	s.Equal(10000, result[0].BasisPoints)
}

func (s *ApplyLeftoverWhiteboxSuite) TestPositiveLeftoverNoOtherAndTargetAbsentIsNoop() {
	target := s.slug("expense.custo_fixo")
	result := []CategoryPercentageInput{}

	applyLeftover(result, target, 10)

	s.Empty(result)
}

func (s *ApplyLeftoverWhiteboxSuite) TestCanonicalRankFallbackForUnknownSlug() {
	order := valueobjects.CanonicalOrder()
	s.Equal(len(order), canonicalRank(valueobjects.RootSlug(0), order))
}
