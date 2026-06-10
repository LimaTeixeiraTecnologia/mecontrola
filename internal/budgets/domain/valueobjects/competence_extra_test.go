package valueobjects_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type CompetenceExtraSuite struct {
	suite.Suite
}

func TestCompetenceExtraSuite(t *testing.T) {
	suite.Run(t, new(CompetenceExtraSuite))
}

func (s *CompetenceExtraSuite) TestCompetenceFromTime() {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	s.NoError(err)
	t := time.Date(2025, 6, 15, 3, 0, 0, 0, time.UTC)
	c := valueobjects.CompetenceFromTime(t, loc)
	s.Equal("2025-06", c.String())
}

func (s *CompetenceExtraSuite) TestIsZero() {
	var zero valueobjects.Competence
	s.True(zero.IsZero())

	c, _ := valueobjects.NewCompetence("2025-06")
	s.False(c.IsZero())
}

func (s *CompetenceExtraSuite) TestSetSaoPauloLocation() {
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	valueobjects.SetSaoPauloLocation(loc)
	s.Equal(loc, valueobjects.SaoPauloLocation())
}
