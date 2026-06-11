package commands_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/commands"
)

type ListAlertsCommandSuite struct {
	suite.Suite
}

func TestListAlertsCommandSuite(t *testing.T) {
	suite.Run(t, new(ListAlertsCommandSuite))
}

func (s *ListAlertsCommandSuite) TestNewListAlertsCommand() {
	validUserID := "11111111-1111-4111-8111-111111111111"

	cases := []struct {
		name      string
		userID    string
		cursor    string
		limit     int
		wantLimit int
		wantErrs  []error
	}{
		{name: "success_default_limit", userID: validUserID, cursor: "", limit: 0, wantLimit: 50},
		{name: "success_clamped_limit", userID: validUserID, cursor: "abc", limit: 500, wantLimit: 200},
		{name: "success_explicit_limit", userID: validUserID, cursor: "abc", limit: 10, wantLimit: 10},
		{name: "invalid_user_id", userID: "x", cursor: "", limit: 0, wantErrs: []error{commands.ErrCommandInvalidUserID}},
		{name: "invalid_limit_negative", userID: validUserID, cursor: "", limit: -1, wantErrs: []error{commands.ErrCommandInvalidLimit}},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			cmd, err := commands.NewListAlertsCommand(tc.userID, tc.cursor, tc.limit)
			if len(tc.wantErrs) == 0 {
				s.Require().NoError(err)
				s.Equal(tc.wantLimit, cmd.Limit)
				s.Equal(tc.cursor, cmd.Cursor)
				return
			}
			s.Require().Error(err)
			for _, w := range tc.wantErrs {
				s.True(errors.Is(err, w))
			}
		})
	}
}
