package services_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type AlertStateResolverSuite struct {
	suite.Suite
}

func TestAlertStateResolverSuite(t *testing.T) {
	suite.Run(t, new(AlertStateResolverSuite))
}

func comp(s *AlertStateResolverSuite, raw string) valueobjects.Competence {
	c, err := valueobjects.NewCompetence(raw)
	s.Require().NoError(err)
	return c
}

func (s *AlertStateResolverSuite) TestResolve() {
	type tc struct {
		name           string
		expense        string
		cutoff         string
		deliveredCount int
		want           entities.AlertState
	}

	cases := []tc{
		{
			name:           "competência atual sem limite — Delivered",
			expense:        "2026-06",
			cutoff:         "2026-06",
			deliveredCount: 0,
			want:           entities.AlertStateDelivered,
		},
		{
			name:           "competência futura — Delivered",
			expense:        "2026-07",
			cutoff:         "2026-06",
			deliveredCount: 0,
			want:           entities.AlertStateDelivered,
		},
		{
			name:           "competência anterior — SuppressedRetroactive",
			expense:        "2026-05",
			cutoff:         "2026-06",
			deliveredCount: 0,
			want:           entities.AlertStateSuppressedRetroactive,
		},
		{
			name:           "count >= max — RateLimited",
			expense:        "2026-06",
			cutoff:         "2026-06",
			deliveredCount: services.MaxDeliveredAlerts,
			want:           entities.AlertStateRateLimited,
		},
		{
			name:           "retroativo com count alto prevalece — SuppressedRetroactive",
			expense:        "2026-05",
			cutoff:         "2026-06",
			deliveredCount: services.MaxDeliveredAlerts + 10,
			want:           entities.AlertStateSuppressedRetroactive,
		},
	}

	resolver := services.NewAlertStateResolver()
	for _, c := range cases {
		s.Run(c.name, func() {
			got := resolver.Resolve(comp(s, c.expense), comp(s, c.cutoff), c.deliveredCount)
			s.Equal(c.want, got)
		})
	}
}
