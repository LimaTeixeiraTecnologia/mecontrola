package commands_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/commands"
)

type DeleteExpenseCommandSuite struct {
	suite.Suite
}

func TestDeleteExpenseCommandSuite(t *testing.T) {
	suite.Run(t, new(DeleteExpenseCommandSuite))
}

func (s *DeleteExpenseCommandSuite) TestNewDeleteExpenseCommand() {
	validUserID := "11111111-1111-4111-8111-111111111111"
	validExtID := "33333333-3333-4333-8333-333333333333"

	cases := []struct {
		name     string
		userID   string
		source   string
		extID    string
		wantErrs []error
	}{
		{name: "success", userID: validUserID, source: "manual", extID: validExtID},
		{name: "invalid_user_id", userID: "x", source: "manual", extID: validExtID, wantErrs: []error{commands.ErrCommandInvalidUserID}},
		{name: "invalid_source", userID: validUserID, source: "", extID: validExtID, wantErrs: []error{commands.ErrCommandInvalidSource}},
		{name: "invalid_ext_id", userID: validUserID, source: "manual", extID: "bad", wantErrs: []error{commands.ErrCommandInvalidExternalID}},
		{name: "all_invalid", userID: "x", source: "", extID: "bad", wantErrs: []error{
			commands.ErrCommandInvalidUserID,
			commands.ErrCommandInvalidSource,
			commands.ErrCommandInvalidExternalID,
		}},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			cmd, err := commands.NewDeleteExpenseCommand(tc.userID, tc.source, tc.extID, 5)
			if len(tc.wantErrs) == 0 {
				s.Require().NoError(err)
				s.Equal(int64(5), cmd.ExpectedVersion)
				return
			}
			s.Require().Error(err)
			for _, w := range tc.wantErrs {
				s.True(errors.Is(err, w))
			}
		})
	}
}
