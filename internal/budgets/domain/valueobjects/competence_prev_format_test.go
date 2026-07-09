package valueobjects_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type CompetencePrevFormatSuite struct {
	suite.Suite
}

func TestCompetencePrevFormatSuite(t *testing.T) {
	suite.Run(t, new(CompetencePrevFormatSuite))
}

func (s *CompetencePrevFormatSuite) TestPrev() {
	type testCase struct {
		name  string
		input string
		want  string
	}

	cases := []testCase{
		{name: "meio de ano", input: "2025-06", want: "2025-05"},
		{name: "virada de ano jan->dez", input: "2025-01", want: "2024-12"},
		{name: "dezembro para novembro", input: "2025-12", want: "2025-11"},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			c, err := valueobjects.NewCompetence(tc.input)
			s.NoError(err)
			s.Equal(tc.want, c.Prev().String())
		})
	}
}

func (s *CompetencePrevFormatSuite) TestPrevNextSymmetry() {
	c, err := valueobjects.NewCompetence("2026-06")
	s.NoError(err)
	s.Equal(c.String(), c.Next().Prev().String())
	s.Equal(c.String(), c.Prev().Next().String())
}

func (s *CompetencePrevFormatSuite) TestFormatCompetencePtBR() {
	type testCase struct {
		name  string
		input string
		want  string
	}

	cases := []testCase{
		{name: "junho de 2026", input: "2026-06", want: "junho de 2026"},
		{name: "janeiro de 2025", input: "2025-01", want: "janeiro de 2025"},
		{name: "dezembro de 2024", input: "2024-12", want: "dezembro de 2024"},
		{name: "todos os meses", input: "2025-03", want: "março de 2025"},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			c, err := valueobjects.NewCompetence(tc.input)
			s.NoError(err)
			s.Equal(tc.want, valueobjects.FormatCompetencePtBR(c))
		})
	}
}

func (s *CompetencePrevFormatSuite) TestFormatCompetencePtBRZero() {
	var zero valueobjects.Competence
	s.Equal("", valueobjects.FormatCompetencePtBR(zero))
}

func (s *CompetencePrevFormatSuite) TestFormatCompetencePtBRAllMonths() {
	months := []string{
		"janeiro", "fevereiro", "março", "abril", "maio", "junho",
		"julho", "agosto", "setembro", "outubro", "novembro", "dezembro",
	}
	for i, name := range months {
		month := i + 1
		raw := ""
		if month < 10 {
			raw = "2030-0" + string(rune('0'+month))
		} else {
			raw = "2030-1" + string(rune('0'+month-10))
		}
		c, err := valueobjects.NewCompetence(raw)
		s.NoError(err)
		s.Equal(name+" de 2030", valueobjects.FormatCompetencePtBR(c))
	}
}
