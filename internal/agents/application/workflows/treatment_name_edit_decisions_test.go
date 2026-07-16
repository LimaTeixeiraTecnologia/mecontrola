package workflows

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type TreatmentNameEditDecisionsSuite struct {
	suite.Suite
	now time.Time
}

func TestTreatmentNameEditDecisionsSuite(t *testing.T) {
	suite.Run(t, new(TreatmentNameEditDecisionsSuite))
}

func (s *TreatmentNameEditDecisionsSuite) SetupTest() {
	s.now = time.Now().UTC()
}

func (s *TreatmentNameEditDecisionsSuite) baseState() TreatmentNameEditState {
	return TreatmentNameEditState{
		Status:       TreatmentNameEditActive,
		ResourceID:   "user-1",
		PreviousName: "Jailton",
		SuspendedAt:  s.now,
	}
}

func (s *TreatmentNameEditDecisionsSuite) TestDecideTreatmentName() {
	scenarios := []struct {
		name       string
		hasName    bool
		raw        string
		expectName string
		expectOK   bool
	}{
		{name: "nome direto", hasName: true, raw: "Stef", expectName: "Stef", expectOK: true},
		{name: "trim bordas", hasName: true, raw: "  Stef  ", expectName: "Stef", expectOK: true},
		{name: "vazio apos trim invalido", hasName: true, raw: "   ", expectName: "", expectOK: false},
		{name: "recusa sem nome invalido", hasName: false, raw: "Stef", expectName: "", expectOK: false},
		{name: "acima de 40 caracteres invalido", hasName: true, raw: strings.Repeat("a", 41), expectName: "", expectOK: false},
		{name: "exatamente 40 caracteres valido", hasName: true, raw: strings.Repeat("a", 40), expectName: strings.Repeat("a", 40), expectOK: true},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			name, ok := DecideTreatmentName(scenario.hasName, scenario.raw)
			s.Equal(scenario.expectName, name)
			s.Equal(scenario.expectOK, ok)
		})
	}
}

func (s *TreatmentNameEditDecisionsSuite) TestDecideTreatmentNameEditExpiry() {
	scenarios := []struct {
		name   string
		state  func() TreatmentNameEditState
		now    time.Time
		expect bool
	}{
		{
			name: "suspendedAt zero nunca expira",
			state: func() TreatmentNameEditState {
				state := s.baseState()
				state.SuspendedAt = time.Time{}
				return state
			},
			now:    s.now,
			expect: false,
		},
		{
			name: "dentro da ttl nao expira",
			state: func() TreatmentNameEditState {
				state := s.baseState()
				state.SuspendedAt = s.now.Add(-10 * time.Minute)
				return state
			},
			now:    s.now,
			expect: false,
		},
		{
			name: "fora da ttl expira",
			state: func() TreatmentNameEditState {
				state := s.baseState()
				state.SuspendedAt = s.now.Add(-16 * time.Minute)
				return state
			},
			now:    s.now,
			expect: true,
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			expired := DecideTreatmentNameEditExpiry(scenario.state(), scenario.now)
			s.Equal(scenario.expect, expired)
		})
	}
}
