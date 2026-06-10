package entities_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type AlertSuite struct {
	suite.Suite
	now        time.Time
	userID     uuid.UUID
	competence valueobjects.Competence
}

func TestAlertSuite(t *testing.T) {
	suite.Run(t, new(AlertSuite))
}

func (s *AlertSuite) SetupTest() {
	s.now = time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	s.userID = uuid.New()
	c, _ := valueobjects.NewCompetence("2025-06")
	s.competence = c
}

func (s *AlertSuite) TestNewAlert() {
	a := entities.NewAlert(
		s.userID,
		s.competence,
		valueobjects.RootSlugCustoFixo,
		valueobjects.Threshold80,
		entities.AlertStatePendingDelivery,
		s.now,
		800,
		1000,
		s.now,
	)
	s.NotEqual(uuid.Nil, a.ID())
	s.Equal(s.userID, a.UserID())
	s.Equal(s.competence, a.Competence())
	s.Equal(valueobjects.RootSlugCustoFixo, a.RootSlug())
	s.Equal(valueobjects.Threshold80, a.Threshold())
	s.Equal(entities.AlertStatePendingDelivery, a.State())
	s.Equal(int64(800), a.SpentCents())
	s.Equal(int64(1000), a.PlannedCents())
	s.True(a.IsVisibleToUser())
}

func (s *AlertSuite) TestIsVisibleToUser() {
	type testCase struct {
		name    string
		state   entities.AlertState
		visible bool
	}

	cases := []testCase{
		{name: "pending_delivery visível", state: entities.AlertStatePendingDelivery, visible: true},
		{name: "delivered visível", state: entities.AlertStateDelivered, visible: true},
		{name: "suppressed_stale invisível", state: entities.AlertStateSuppressedStale, visible: false},
		{name: "suppressed_retroactive invisível", state: entities.AlertStateSuppressedRetroactive, visible: false},
		{name: "rate_limited invisível", state: entities.AlertStateRateLimited, visible: false},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			a := entities.NewAlert(
				s.userID, s.competence, valueobjects.RootSlugMetas,
				valueobjects.Threshold100, tc.state, s.now, 0, 0, s.now,
			)
			s.Equal(tc.visible, a.IsVisibleToUser())
		})
	}
}

func (s *AlertSuite) TestHydrateAlert() {
	id := uuid.New()
	a := entities.HydrateAlert(
		id, s.userID, s.competence,
		valueobjects.RootSlugPrazeres, valueobjects.Threshold100,
		entities.AlertStateDelivered, s.now, 1000, 1000, s.now,
	)
	s.Equal(id, a.ID())
	s.Equal(entities.AlertStateDelivered, a.State())
	s.Equal(s.now, a.TriggeredByCommittedAt())
	s.Equal(s.now, a.CreatedAt())
}
