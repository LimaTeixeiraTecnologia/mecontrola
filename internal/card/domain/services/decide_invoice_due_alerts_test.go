package services_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
)

type InvoiceDueAlertsDeciderSuite struct {
	suite.Suite
	decider services.InvoiceDueAlertsDecider
	loc     *time.Location
}

func TestInvoiceDueAlertsDeciderSuite(t *testing.T) {
	suite.Run(t, new(InvoiceDueAlertsDeciderSuite))
}

func (s *InvoiceDueAlertsDeciderSuite) SetupTest() {
	s.decider = services.NewInvoiceDueAlertsDecider()
	s.loc = time.UTC
}

func (s *InvoiceDueAlertsDeciderSuite) candidate(dueDay int) services.InvoiceDueCandidate {
	cycle, err := valueobjects.NewBillingCycle(1, dueDay)
	s.Require().NoError(err)
	return services.InvoiceDueCandidate{
		UserID:       uuid.New(),
		CardID:       uuid.New(),
		CardNickname: "Card",
		Cycle:        cycle,
	}
}

func (s *InvoiceDueAlertsDeciderSuite) TestDecide_WithinWindow() {
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	c := s.candidate(13)

	alerts := s.decider.Decide([]services.InvoiceDueCandidate{c}, 3, now, s.loc)
	s.Require().Len(alerts, 1)
	s.Equal(3, alerts[0].DaysUntil)
	s.Equal("2026-06-13", alerts[0].DueDate.Format("2006-01-02"))
}

func (s *InvoiceDueAlertsDeciderSuite) TestDecide_OutsideWindow() {
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	c := s.candidate(20)

	alerts := s.decider.Decide([]services.InvoiceDueCandidate{c}, 3, now, s.loc)
	s.Empty(alerts)
}

func (s *InvoiceDueAlertsDeciderSuite) TestDecide_DueToday() {
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	c := s.candidate(10)

	alerts := s.decider.Decide([]services.InvoiceDueCandidate{c}, 3, now, s.loc)
	s.Require().Len(alerts, 1)
	s.Equal(0, alerts[0].DaysUntil)
}

func (s *InvoiceDueAlertsDeciderSuite) TestDecide_MonthBoundaryRollsToNextMonth() {
	now := time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)
	c := s.candidate(1)

	alerts := s.decider.Decide([]services.InvoiceDueCandidate{c}, 3, now, s.loc)
	s.Require().Len(alerts, 1)
	s.Equal("2026-07-01", alerts[0].DueDate.Format("2006-01-02"))
	s.Equal(2, alerts[0].DaysUntil)
}
