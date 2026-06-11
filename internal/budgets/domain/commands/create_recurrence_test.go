package commands_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/commands"
)

type CreateRecurrenceCommandSuite struct {
	suite.Suite
}

func TestCreateRecurrenceCommandSuite(t *testing.T) {
	suite.Run(t, new(CreateRecurrenceCommandSuite))
}

func (s *CreateRecurrenceCommandSuite) TestNewCreateRecurrenceCommand() {
	validUserID := "11111111-1111-4111-8111-111111111111"

	cases := []struct {
		name       string
		userID     string
		competence string
		months     int
		wantErrs   []error
	}{
		{name: "success", userID: validUserID, competence: "2026-06", months: 3},
		{name: "invalid_user_id", userID: "x", competence: "2026-06", months: 3, wantErrs: []error{commands.ErrCommandInvalidUserID}},
		{name: "invalid_competence", userID: validUserID, competence: "bad", months: 3, wantErrs: []error{commands.ErrCommandInvalidCompetence}},
		{name: "months_zero", userID: validUserID, competence: "2026-06", months: 0, wantErrs: []error{commands.ErrCommandInvalidMonths}},
		{name: "months_too_many", userID: validUserID, competence: "2026-06", months: 13, wantErrs: []error{commands.ErrCommandInvalidMonths}},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			cmd, err := commands.NewCreateRecurrenceCommand(tc.userID, tc.competence, tc.months)
			if len(tc.wantErrs) == 0 {
				s.Require().NoError(err)
				s.Equal(tc.months, cmd.Months)
				return
			}
			s.Require().Error(err)
			for _, w := range tc.wantErrs {
				s.True(errors.Is(err, w))
			}
		})
	}
}
