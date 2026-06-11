package mappers_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/mappers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type AlertMapperSuite struct {
	suite.Suite
}

func TestAlertMapperSuite(t *testing.T) {
	suite.Run(t, new(AlertMapperSuite))
}

func (s *AlertMapperSuite) buildAlert(state entities.AlertState) (entities.Alert, uuid.UUID, uuid.UUID, time.Time, time.Time) {
	id := uuid.New()
	userID := uuid.New()
	competence, err := valueobjects.NewCompetence("2026-06")
	s.Require().NoError(err)
	rootSlug, err := valueobjects.ParseRootSlug("expense.custo_fixo")
	s.Require().NoError(err)
	threshold, err := valueobjects.ParseThreshold(80)
	s.Require().NoError(err)
	triggered := time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC)
	createdAt := time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC)
	a := entities.HydrateAlert(id, userID, competence, rootSlug, threshold, state, triggered, 8000, 10000, createdAt)
	return a, id, userID, triggered, createdAt
}

func (s *AlertMapperSuite) TestAlert() {
	a, id, userID, triggered, createdAt := s.buildAlert(entities.AlertStateDelivered)
	out := mappers.M.Alert(a)
	s.Equal(id.String(), out.ID)
	s.Equal(userID.String(), out.UserID)
	s.Equal("2026-06", out.Competence)
	s.Equal("expense.custo_fixo", out.RootSlug)
	s.Equal(80, out.Threshold)
	s.Equal("delivered", out.State)
	s.Equal(triggered, out.TriggeredByCommittedAt)
	s.Equal(int64(8000), out.SpentCents)
	s.Equal(int64(10000), out.PlannedCents)
	s.Equal(createdAt, out.CreatedAt)
}

func (s *AlertMapperSuite) TestAlerts() {
	a1, _, _, _, _ := s.buildAlert(entities.AlertStatePendingDelivery)
	a2, _, _, _, _ := s.buildAlert(entities.AlertStateDelivered)
	outs := mappers.M.Alerts([]entities.Alert{a1, a2})
	s.Len(outs, 2)
	s.Equal("pending_delivery", outs[0].State)
	s.Equal("delivered", outs[1].State)
}

func (s *AlertMapperSuite) TestListAlerts() {
	a, _, _, _, _ := s.buildAlert(entities.AlertStateRateLimited)
	out := mappers.M.ListAlerts([]entities.Alert{a}, "next-cursor")
	s.Len(out.Alerts, 1)
	s.Equal("rate_limited", out.Alerts[0].State)
	s.Equal("next-cursor", out.NextCursor)
}

func (s *AlertMapperSuite) TestAlertStateString() {
	tests := []struct {
		name  string
		state entities.AlertState
		want  string
	}{
		{name: "pending_delivery", state: entities.AlertStatePendingDelivery, want: "pending_delivery"},
		{name: "delivered", state: entities.AlertStateDelivered, want: "delivered"},
		{name: "suppressed_stale", state: entities.AlertStateSuppressedStale, want: "suppressed_stale"},
		{name: "suppressed_retroactive", state: entities.AlertStateSuppressedRetroactive, want: "suppressed_retroactive"},
		{name: "rate_limited", state: entities.AlertStateRateLimited, want: "rate_limited"},
		{name: "desconhecido", state: entities.AlertState(0), want: ""},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.Equal(tt.want, mappers.M.AlertStateString(tt.state))
		})
	}
}
