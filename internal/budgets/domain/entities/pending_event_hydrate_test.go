package entities_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type PendingEventHydrateSuite struct {
	suite.Suite
	now time.Time
}

func TestPendingEventHydrateSuite(t *testing.T) {
	suite.Run(t, new(PendingEventHydrateSuite))
}

func (s *PendingEventHydrateSuite) SetupTest() {
	s.now = time.Date(2025, 6, 1, 10, 0, 0, 0, time.UTC)
}

func (s *PendingEventHydrateSuite) TestHydratePendingEvent() {
	id := uuid.New()
	eventID := uuid.New()
	userID := uuid.New()
	src, _ := valueobjects.NewProducerSource("billing")
	ext, _ := valueobjects.NewExternalTransactionID("f47ac10b-58cc-4372-a567-0e02b2c3d479")
	transitioned := s.now.Add(time.Hour)

	p := entities.HydratePendingEvent(
		id, eventID, src, userID, ext, 2,
		valueobjects.MutationKindUpdate,
		[]byte(`{"k":"v"}`),
		entities.PendingStateApplied,
		s.now, &transitioned, "applied ok",
	)

	s.Equal(id, p.ID())
	s.Equal(eventID, p.EventID())
	s.Equal(src, p.Source())
	s.Equal(userID, p.UserID())
	s.Equal(ext, p.ExternalTransactionID())
	s.Equal(int64(2), p.ExpectedVersion())
	s.Equal(valueobjects.MutationKindUpdate, p.MutationKind())
	s.Equal([]byte(`{"k":"v"}`), p.Payload())
	s.Equal(entities.PendingStateApplied, p.State())
	s.Equal(s.now, p.ReceivedAt())
	s.NotNil(p.TransitionedAt())
	s.Equal("applied ok", p.Reason())
	s.True(p.IsTerminal())
}
