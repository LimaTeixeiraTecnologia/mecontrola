package commands_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/commands"
)

type GetMonthlySummaryCommandSuite struct {
	suite.Suite
}

func TestGetMonthlySummaryCommandSuite(t *testing.T) {
	suite.Run(t, new(GetMonthlySummaryCommandSuite))
}

func (s *GetMonthlySummaryCommandSuite) TestNewGetMonthlySummaryCommand() {
	validUserID := "11111111-1111-4111-8111-111111111111"

	cases := []struct {
		name       string
		userID     string
		competence string
		wantErrs   []error
	}{
		{name: "success", userID: validUserID, competence: "2026-06"},
		{name: "invalid_user_id", userID: "x", competence: "2026-06", wantErrs: []error{commands.ErrCommandInvalidUserID}},
		{name: "invalid_competence", userID: validUserID, competence: "bad", wantErrs: []error{commands.ErrCommandInvalidCompetence}},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			_, err := commands.NewGetMonthlySummaryCommand(tc.userID, tc.competence)
			if len(tc.wantErrs) == 0 {
				s.Require().NoError(err)
				return
			}
			s.Require().Error(err)
			for _, w := range tc.wantErrs {
				s.True(errors.Is(err, w))
			}
		})
	}
}
