package commands_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/commands"
)

type ApplyPendingEventCommandSuite struct {
	suite.Suite
}

func TestApplyPendingEventCommandSuite(t *testing.T) {
	suite.Run(t, new(ApplyPendingEventCommandSuite))
}

func (s *ApplyPendingEventCommandSuite) TestNewApplyPendingEventCommand() {
	cases := []struct {
		name    string
		eventID string
		wantErr error
	}{
		{name: "success", eventID: "44444444-4444-4444-8444-444444444444"},
		{name: "invalid_event_id", eventID: "bad", wantErr: commands.ErrCommandInvalidEventID},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			cmd, err := commands.NewApplyPendingEventCommand(tc.eventID)
			if tc.wantErr == nil {
				s.Require().NoError(err)
				s.NotEqual("", cmd.EventID.String())
				return
			}
			s.Require().Error(err)
			s.True(errors.Is(err, tc.wantErr))
		})
	}
}
