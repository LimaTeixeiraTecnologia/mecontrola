package valueobjects_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type CompetenceSuite struct {
	suite.Suite
}

func TestCompetenceSuite(t *testing.T) {
	suite.Run(t, new(CompetenceSuite))
}

func (s *CompetenceSuite) TestNewCompetence() {
	type testCase struct {
		name    string
		input   string
		wantErr bool
	}

	cases := []testCase{
		{name: "válido janeiro", input: "2025-01", wantErr: false},
		{name: "válido dezembro", input: "2025-12", wantErr: false},
		{name: "muito curto", input: "2025-1", wantErr: true},
		{name: "muito longo", input: "2025-011", wantErr: true},
		{name: "mês 13", input: "2025-13", wantErr: true},
		{name: "mês 00", input: "2025-00", wantErr: true},
		{name: "separador errado", input: "2025/01", wantErr: true},
		{name: "letras", input: "20XX-01", wantErr: true},
		{name: "vazio", input: "", wantErr: true},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			got, err := valueobjects.NewCompetence(tc.input)
			if tc.wantErr {
				s.Error(err)
				return
			}
			s.NoError(err)
			s.Equal(tc.input, got.String())
		})
	}
}

func (s *CompetenceSuite) TestBefore() {
	a, _ := valueobjects.NewCompetence("2025-01")
	b, _ := valueobjects.NewCompetence("2025-02")
	s.True(a.Before(b))
	s.False(b.Before(a))
	s.False(a.Before(a))
}

func (s *CompetenceSuite) TestEqual() {
	a, _ := valueobjects.NewCompetence("2025-06")
	b, _ := valueobjects.NewCompetence("2025-06")
	c, _ := valueobjects.NewCompetence("2025-07")
	s.True(a.Equal(b))
	s.False(a.Equal(c))
}

func (s *CompetenceSuite) TestNext() {
	c, _ := valueobjects.NewCompetence("2025-11")
	s.Equal("2025-12", c.Next().String())
	c2, _ := valueobjects.NewCompetence("2025-12")
	s.Equal("2026-01", c2.Next().String())
}
