package commands_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/commands"
)

type EvaluateAlertCommandSuite struct {
	suite.Suite
}

func TestEvaluateAlertCommandSuite(t *testing.T) {
	suite.Run(t, new(EvaluateAlertCommandSuite))
}

func (s *EvaluateAlertCommandSuite) TestNewEvaluateAlertCommand() {
	validUserID := "11111111-1111-4111-8111-111111111111"
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name       string
		userID     string
		competence string
		wantErrs   []error
	}{
		{name: "success", userID: validUserID, competence: "2026-06"},
		{name: "invalid_user_id", userID: "x", competence: "2026-06", wantErrs: []error{commands.ErrCommandInvalidUserID}},
		{name: "invalid_competence", userID: validUserID, competence: "bad", wantErrs: []error{commands.ErrCommandInvalidCompetence}},
		{name: "all_invalid", userID: "x", competence: "bad", wantErrs: []error{commands.ErrCommandInvalidUserID, commands.ErrCommandInvalidCompetence}},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			cmd, err := commands.NewEvaluateAlertCommand(tc.userID, tc.competence, now)
			if len(tc.wantErrs) == 0 {
				s.Require().NoError(err)
				s.Equal(now.UTC(), cmd.NowUTC)
				return
			}
			s.Require().Error(err)
			for _, w := range tc.wantErrs {
				s.True(errors.Is(err, w))
			}
		})
	}
}
