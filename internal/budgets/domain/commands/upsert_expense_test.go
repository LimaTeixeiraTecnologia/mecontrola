package commands_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/commands"
)

type UpsertExpenseCommandSuite struct {
	suite.Suite
}

func TestUpsertExpenseCommandSuite(t *testing.T) {
	suite.Run(t, new(UpsertExpenseCommandSuite))
}

func (s *UpsertExpenseCommandSuite) TestNewUpsertExpenseCommand() {
	validUserID := "11111111-1111-4111-8111-111111111111"
	validSubID := "22222222-2222-4222-8222-222222222222"
	validExtID := "33333333-3333-4333-8333-333333333333"
	expectedV := int64(1)
	invalidV := int64(0)
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name            string
		userID          string
		subID           string
		source          string
		extID           string
		competence      string
		amountCents     int64
		occurredAt      time.Time
		expectedVersion *int64
		wantErrs        []error
	}{
		{
			name:            "success_create",
			userID:          validUserID,
			subID:           validSubID,
			source:          "manual",
			extID:           validExtID,
			competence:      "2026-06",
			amountCents:     1000,
			occurredAt:      now,
			expectedVersion: nil,
		},
		{
			name:            "success_update",
			userID:          validUserID,
			subID:           validSubID,
			source:          "manual",
			extID:           validExtID,
			competence:      "2026-06",
			amountCents:     1000,
			occurredAt:      now,
			expectedVersion: &expectedV,
		},
		{
			name:   "invalid_user_id",
			userID: "not-a-uuid",
			subID:  validSubID,
			source: "manual", extID: validExtID, competence: "2026-06", amountCents: 1, occurredAt: now,
			wantErrs: []error{commands.ErrCommandInvalidUserID},
		},
		{
			name:   "invalid_subcategory",
			userID: validUserID,
			subID:  "x",
			source: "manual", extID: validExtID, competence: "2026-06", amountCents: 1, occurredAt: now,
			wantErrs: []error{commands.ErrCommandInvalidSubcategory},
		},
		{
			name:   "invalid_source",
			userID: validUserID, subID: validSubID, source: "", extID: validExtID,
			competence: "2026-06", amountCents: 1, occurredAt: now,
			wantErrs: []error{commands.ErrCommandInvalidSource},
		},
		{
			name:   "invalid_ext_id",
			userID: validUserID, subID: validSubID, source: "manual", extID: "bad",
			competence: "2026-06", amountCents: 1, occurredAt: now,
			wantErrs: []error{commands.ErrCommandInvalidExternalID},
		},
		{
			name:   "invalid_competence",
			userID: validUserID, subID: validSubID, source: "manual", extID: validExtID,
			competence: "2026/06", amountCents: 1, occurredAt: now,
			wantErrs: []error{commands.ErrCommandInvalidCompetence},
		},
		{
			name:   "invalid_amount",
			userID: validUserID, subID: validSubID, source: "manual", extID: validExtID,
			competence: "2026-06", amountCents: 0, occurredAt: now,
			wantErrs: []error{commands.ErrCommandInvalidAmount},
		},
		{
			name:   "invalid_expected_version",
			userID: validUserID, subID: validSubID, source: "manual", extID: validExtID,
			competence: "2026-06", amountCents: 1, occurredAt: now,
			expectedVersion: &invalidV,
			wantErrs:        []error{commands.ErrCommandVersionRequired},
		},
		{
			name:   "multiple_errors_aggregated",
			userID: "x", subID: "y", source: "", extID: "bad",
			competence: "bad", amountCents: 0, occurredAt: now,
			wantErrs: []error{
				commands.ErrCommandInvalidUserID,
				commands.ErrCommandInvalidSubcategory,
				commands.ErrCommandInvalidSource,
				commands.ErrCommandInvalidExternalID,
				commands.ErrCommandInvalidCompetence,
				commands.ErrCommandInvalidAmount,
			},
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			cmd, err := commands.NewUpsertExpenseCommand(tc.userID, tc.subID, tc.source, tc.extID, tc.competence, tc.amountCents, tc.occurredAt, tc.expectedVersion)
			if len(tc.wantErrs) == 0 {
				s.Require().NoError(err)
				s.Equal(tc.amountCents, cmd.AmountCents)
				return
			}
			s.Require().Error(err)
			for _, want := range tc.wantErrs {
				s.True(errors.Is(err, want), "expected %v in %v", want, err)
			}
		})
	}
}
