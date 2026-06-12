package entities_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type PendingEventSuite struct {
	suite.Suite
	now time.Time
}

func TestPendingEventSuite(t *testing.T) {
	suite.Run(t, new(PendingEventSuite))
}

func (s *PendingEventSuite) SetupTest() {
	s.now = time.Date(2025, 6, 1, 10, 0, 0, 0, time.UTC)
}

func (s *PendingEventSuite) newPendingEvent() entities.PendingEvent {
	src, _ := valueobjects.NewProducerSource("billing")
	ext, _ := valueobjects.NewExternalTransactionID("f47ac10b-58cc-4372-a567-0e02b2c3d479")
	return entities.NewPendingEvent(
		uuid.New(),
		src,
		uuid.New(),
		ext,
		1,
		valueobjects.MutationKindCreate,
		[]byte(`{}`),
		s.now,
	)
}

func (s *PendingEventSuite) TestInitialState() {
	p := s.newPendingEvent()
	s.Equal(entities.PendingStatePending, p.State())
	s.False(p.IsTerminal())
}

func (s *PendingEventSuite) TestTransition() {
	type testCase struct {
		name    string
		to      entities.PendingState
		wantErr bool
	}

	cases := []testCase{
		{name: "pending -> applied", to: entities.PendingStateApplied, wantErr: false},
		{name: "pending -> failed", to: entities.PendingStateFailed, wantErr: false},
		{name: "pending -> expired", to: entities.PendingStateExpired, wantErr: false},
		{name: "pending -> pending inválido", to: entities.PendingStatePending, wantErr: true},
		{name: "pending -> zero inválido", to: entities.PendingState(0), wantErr: true},
		{name: "pending -> valor fora do enum inválido", to: entities.PendingState(99), wantErr: true},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			p := s.newPendingEvent()
			err := p.Transition(tc.to, "motivo", s.now)
			if tc.wantErr {
				s.Error(err)
				return
			}
			s.NoError(err)
			s.Equal(tc.to, p.State())
			s.True(p.IsTerminal())
		})
	}
}

func (s *PendingEventSuite) TestTransitionFromTerminalFails() {
	p := s.newPendingEvent()
	_ = p.Transition(entities.PendingStateApplied, "ok", s.now)
	err := p.Transition(entities.PendingStateExpired, "tarde", s.now)
	s.ErrorIs(err, entities.ErrPendingStateTransitionInvalid)
}

func (s *PendingEventSuite) TestIsExpired() {
	p := s.newPendingEvent()
	ttl := 24 * time.Hour
	s.False(p.IsExpired(ttl, s.now.Add(23*time.Hour)))
	s.True(p.IsExpired(ttl, s.now.Add(25*time.Hour)))
}
