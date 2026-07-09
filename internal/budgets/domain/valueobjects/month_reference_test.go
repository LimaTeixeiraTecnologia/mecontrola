package valueobjects_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type MonthReferenceSuite struct {
	suite.Suite
}

func TestMonthReferenceSuite(t *testing.T) {
	suite.Run(t, new(MonthReferenceSuite))
}

func (s *MonthReferenceSuite) TestMonthRefKindStringIsValidParse() {
	kinds := []valueobjects.MonthRefKind{
		valueobjects.MonthRefCurrent,
		valueobjects.MonthRefPrevious,
		valueobjects.MonthRefNext,
		valueobjects.MonthRefExplicit,
		valueobjects.MonthRefNamedWithoutYear,
		valueobjects.MonthRefUnknown,
	}

	for _, k := range kinds {
		s.Run(k.String(), func() {
			s.True(k.IsValid())
			parsed, err := valueobjects.ParseMonthRefKind(k.String())
			s.NoError(err)
			s.Equal(k, parsed)
		})
	}

	invalid := valueobjects.MonthRefKind(99)
	s.False(invalid.IsValid())
	s.Equal("", invalid.String())

	_, err := valueobjects.ParseMonthRefKind("invalido")
	s.ErrorIs(err, valueobjects.ErrMonthRefKindUnknown)
}

func (s *MonthReferenceSuite) TestClarifyReasonStringIsValidParse() {
	reasons := []valueobjects.ClarifyReason{
		valueobjects.ClarifyNone,
		valueobjects.ClarifyMissingYear,
		valueobjects.ClarifyUnrecognized,
	}

	for _, r := range reasons {
		s.Run(r.String(), func() {
			s.True(r.IsValid())
			parsed, err := valueobjects.ParseClarifyReason(r.String())
			s.NoError(err)
			s.Equal(r, parsed)
		})
	}

	invalid := valueobjects.ClarifyReason(99)
	s.False(invalid.IsValid())
	s.Equal("", invalid.String())

	_, err := valueobjects.ParseClarifyReason("invalido")
	s.ErrorIs(err, valueobjects.ErrClarifyReasonUnknown)
}

func (s *MonthReferenceSuite) TestDecideCompetence() {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	s.Require().NoError(err)

	type testCase struct {
		name           string
		ref            valueobjects.MonthReference
		now            time.Time
		wantCompetence string
		wantClarify    valueobjects.ClarifyReason
		wantErr        bool
	}

	cases := []testCase{
		{
			name:           "mês atual meio de ano",
			ref:            valueobjects.MonthReference{Kind: valueobjects.MonthRefCurrent},
			now:            time.Date(2026, 6, 15, 10, 0, 0, 0, loc),
			wantCompetence: "2026-06",
			wantClarify:    valueobjects.ClarifyNone,
		},
		{
			name:           "mês passado dentro do mesmo ano",
			ref:            valueobjects.MonthReference{Kind: valueobjects.MonthRefPrevious},
			now:            time.Date(2026, 6, 15, 10, 0, 0, 0, loc),
			wantCompetence: "2026-05",
			wantClarify:    valueobjects.ClarifyNone,
		},
		{
			name:           "mês passado virada de ano jan->dez",
			ref:            valueobjects.MonthReference{Kind: valueobjects.MonthRefPrevious},
			now:            time.Date(2026, 1, 10, 10, 0, 0, 0, loc),
			wantCompetence: "2025-12",
			wantClarify:    valueobjects.ClarifyNone,
		},
		{
			name:           "mês que vem dentro do mesmo ano",
			ref:            valueobjects.MonthReference{Kind: valueobjects.MonthRefNext},
			now:            time.Date(2026, 6, 15, 10, 0, 0, 0, loc),
			wantCompetence: "2026-07",
			wantClarify:    valueobjects.ClarifyNone,
		},
		{
			name:           "mês que vem virada de ano dez->jan",
			ref:            valueobjects.MonthReference{Kind: valueobjects.MonthRefNext},
			now:            time.Date(2026, 12, 20, 10, 0, 0, 0, loc),
			wantCompetence: "2027-01",
			wantClarify:    valueobjects.ClarifyNone,
		},
		{
			name:           "explicit junho de 2026",
			ref:            valueobjects.MonthReference{Kind: valueobjects.MonthRefExplicit, Year: 2026, Month: 6},
			now:            time.Date(2026, 1, 1, 0, 0, 0, 0, loc),
			wantCompetence: "2026-06",
			wantClarify:    valueobjects.ClarifyNone,
		},
		{
			name:           "explicit janeiro de 2025 retroativo",
			ref:            valueobjects.MonthReference{Kind: valueobjects.MonthRefExplicit, Year: 2025, Month: 1},
			now:            time.Date(2026, 7, 9, 0, 0, 0, 0, loc),
			wantCompetence: "2025-01",
			wantClarify:    valueobjects.ClarifyNone,
		},
		{
			name:    "explicit inválido mês 13",
			ref:     valueobjects.MonthReference{Kind: valueobjects.MonthRefExplicit, Year: 2026, Month: 13},
			now:     time.Date(2026, 1, 1, 0, 0, 0, 0, loc),
			wantErr: true,
		},
		{
			name:        "explicit sem ano pede esclarecimento em vez de zerar",
			ref:         valueobjects.MonthReference{Kind: valueobjects.MonthRefExplicit, Year: 0, Month: 6},
			now:         time.Date(2026, 7, 9, 0, 0, 0, 0, loc),
			wantClarify: valueobjects.ClarifyMissingYear,
		},
		{
			name:        "explicit ano negativo pede esclarecimento",
			ref:         valueobjects.MonthReference{Kind: valueobjects.MonthRefExplicit, Year: -1, Month: 6},
			now:         time.Date(2026, 7, 9, 0, 0, 0, 0, loc),
			wantClarify: valueobjects.ClarifyMissingYear,
		},
		{
			name:        "nomeado sem ano pede esclarecimento",
			ref:         valueobjects.MonthReference{Kind: valueobjects.MonthRefNamedWithoutYear, Month: 6},
			now:         time.Date(2026, 1, 1, 0, 0, 0, 0, loc),
			wantClarify: valueobjects.ClarifyMissingYear,
		},
		{
			name:        "sem referência reconhecível",
			ref:         valueobjects.MonthReference{Kind: valueobjects.MonthRefUnknown},
			now:         time.Date(2026, 1, 1, 0, 0, 0, 0, loc),
			wantClarify: valueobjects.ClarifyUnrecognized,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			c, clarify, err := valueobjects.DecideCompetence(tc.ref, tc.now)
			if tc.wantErr {
				s.Error(err)
				return
			}
			s.NoError(err)
			s.Equal(tc.wantClarify, clarify)
			if tc.wantCompetence != "" {
				s.Equal(tc.wantCompetence, c.String())
			} else {
				s.True(c.IsZero())
			}
		})
	}
}

func (s *MonthReferenceSuite) TestDecideCompetenceInvalidKind() {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	s.Require().NoError(err)

	_, _, err = valueobjects.DecideCompetence(valueobjects.MonthReference{Kind: valueobjects.MonthRefKind(99)}, time.Date(2026, 1, 1, 0, 0, 0, 0, loc))
	s.ErrorIs(err, valueobjects.ErrMonthRefKindUnknown)
}
