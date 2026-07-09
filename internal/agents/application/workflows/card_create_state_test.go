package workflows

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type CardCreateStateSuite struct {
	suite.Suite
}

func TestCardCreateStateSuite(t *testing.T) {
	suite.Run(t, new(CardCreateStateSuite))
}

func (s *CardCreateStateSuite) TestCardCreateStatusRoundTrip() {
	type args struct {
		status CardCreateStatus
		text   string
	}

	scenarios := []struct {
		name string
		args args
	}{
		{name: "active", args: args{status: CardCreateStatusActive, text: "active"}},
		{name: "completed", args: args{status: CardCreateStatusCompleted, text: "completed"}},
		{name: "cancelled", args: args{status: CardCreateStatusCancelled, text: "cancelled"}},
		{name: "expired", args: args{status: CardCreateStatusExpired, text: "expired"}},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.Equal(scenario.args.text, scenario.args.status.String())
			s.True(scenario.args.status.IsValid())

			parsed, err := ParseCardCreateStatus(scenario.args.text)
			s.NoError(err)
			s.Equal(scenario.args.status, parsed)
		})
	}
}

func (s *CardCreateStateSuite) TestParseCardCreateStatusInvalid() {
	parsed, err := ParseCardCreateStatus("nao-existe")
	s.Error(err)
	s.Zero(parsed)
}

func (s *CardCreateStateSuite) TestCardCreateStatusIsValidRejectsOutOfRange() {
	s.False(CardCreateStatus(0).IsValid())
	s.False(CardCreateStatus(99).IsValid())
}

func (s *CardCreateStateSuite) TestCardCreateStatusStringUnknown() {
	s.Equal("unknown", CardCreateStatus(0).String())
	s.Equal("unknown", CardCreateStatus(99).String())
}
