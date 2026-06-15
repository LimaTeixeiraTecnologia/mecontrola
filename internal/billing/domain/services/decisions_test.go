package services_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

type DecisionsSuite struct {
	suite.Suite
	svc services.TransitionService
	now time.Time
}

func TestDecisionsSuite(t *testing.T) {
	suite.Run(t, new(DecisionsSuite))
}

func (s *DecisionsSuite) SetupTest() {
	s.svc = services.NewTransitionService()
	s.now = time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
}

func (s *DecisionsSuite) TestDecideRenewal() {
	type tc struct {
		name        string
		current     valueobjects.Status
		occurredAt  time.Time
		lastEventAt time.Time
		want        services.Decision
	}

	cases := []tc{
		{
			name:        "aplica evento recente",
			current:     valueobjects.StatusPastDue,
			occurredAt:  s.now,
			lastEventAt: s.now.Add(-1 * time.Hour),
			want:        services.DecisionApply,
		},
		{
			name:        "aplica primeiro evento sem lastEventAt",
			current:     valueobjects.StatusPastDue,
			occurredAt:  s.now,
			lastEventAt: time.Time{},
			want:        services.DecisionApply,
		},
		{
			name:        "suprime regressao de evento atrasado com transicao valida",
			current:     valueobjects.StatusPastDue,
			occurredAt:  s.now.Add(-2 * time.Hour),
			lastEventAt: s.now,
			want:        services.DecisionSkipAsRegression,
		},
		{
			name:        "aplica mesmo com timestamp antigo quando nao ha transicao valida",
			current:     valueobjects.StatusActive,
			occurredAt:  s.now.Add(-2 * time.Hour),
			lastEventAt: s.now,
			want:        services.DecisionApply,
		},
	}

	for _, c := range cases {
		s.Run(c.name, func() {
			got := s.svc.DecideRenewal(c.current, c.occurredAt, c.lastEventAt)
			s.Equal(c.want, got)
		})
	}
}

func (s *DecisionsSuite) TestDecidePastDue() {
	type tc struct {
		name        string
		current     valueobjects.Status
		occurredAt  time.Time
		lastEventAt time.Time
		want        services.Decision
	}

	cases := []tc{
		{
			name:        "aplica evento recente",
			current:     valueobjects.StatusActive,
			occurredAt:  s.now,
			lastEventAt: s.now.Add(-1 * time.Hour),
			want:        services.DecisionApply,
		},
		{
			name:        "aplica primeiro evento sem lastEventAt",
			current:     valueobjects.StatusActive,
			occurredAt:  s.now,
			lastEventAt: time.Time{},
			want:        services.DecisionApply,
		},
		{
			name:        "suprime regressao de evento atrasado com transicao valida",
			current:     valueobjects.StatusActive,
			occurredAt:  s.now.Add(-2 * time.Hour),
			lastEventAt: s.now,
			want:        services.DecisionSkipAsRegression,
		},
		{
			name:        "aplica mesmo com timestamp antigo quando status nao pode transicionar",
			current:     valueobjects.StatusExpired,
			occurredAt:  s.now.Add(-2 * time.Hour),
			lastEventAt: s.now,
			want:        services.DecisionApply,
		},
	}

	for _, c := range cases {
		s.Run(c.name, func() {
			got := s.svc.DecidePastDue(c.current, c.occurredAt, c.lastEventAt)
			s.Equal(c.want, got)
		})
	}
}

func (s *DecisionsSuite) TestDecideCancellation() {
	type tc struct {
		name        string
		current     valueobjects.Status
		occurredAt  time.Time
		lastEventAt time.Time
		want        services.Decision
	}

	cases := []tc{
		{
			name:        "aplica evento recente",
			current:     valueobjects.StatusActive,
			occurredAt:  s.now,
			lastEventAt: s.now.Add(-1 * time.Hour),
			want:        services.DecisionApply,
		},
		{
			name:        "aplica primeiro evento sem lastEventAt",
			current:     valueobjects.StatusActive,
			occurredAt:  s.now,
			lastEventAt: time.Time{},
			want:        services.DecisionApply,
		},
		{
			name:        "suprime regressao de evento atrasado com transicao valida",
			current:     valueobjects.StatusActive,
			occurredAt:  s.now.Add(-2 * time.Hour),
			lastEventAt: s.now,
			want:        services.DecisionSkipAsRegression,
		},
		{
			name:        "aplica mesmo com timestamp antigo quando status nao pode transicionar",
			current:     valueobjects.StatusRefunded,
			occurredAt:  s.now.Add(-2 * time.Hour),
			lastEventAt: s.now,
			want:        services.DecisionApply,
		},
	}

	for _, c := range cases {
		s.Run(c.name, func() {
			got := s.svc.DecideCancellation(c.current, c.occurredAt, c.lastEventAt)
			s.Equal(c.want, got)
		})
	}
}
