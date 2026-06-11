package commands_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/commands"
)

type IngestExternalExpenseCommandSuite struct {
	suite.Suite
}

func TestIngestExternalExpenseCommandSuite(t *testing.T) {
	suite.Run(t, new(IngestExternalExpenseCommandSuite))
}

func (s *IngestExternalExpenseCommandSuite) TestNewIngestExternalExpenseCommand() {
	validEventID := "44444444-4444-4444-8444-444444444444"
	validUserID := "11111111-1111-4111-8111-111111111111"
	validSubID := "22222222-2222-4222-8222-222222222222"
	validExtID := "33333333-3333-4333-8333-333333333333"
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name        string
		eventID     string
		userID      string
		source      string
		extID       string
		subID       string
		competence  string
		operation   string
		version     int64
		amountCents int64
		occurredAt  time.Time
		wantErrs    []error
	}{
		{
			name:    "success_create",
			eventID: validEventID, userID: validUserID, source: "kiwify", extID: validExtID,
			subID: validSubID, competence: "2026-06", operation: "create",
			version: 1, amountCents: 100, occurredAt: now,
		},
		{
			name:    "success_delete_no_amount",
			eventID: validEventID, userID: validUserID, source: "kiwify", extID: validExtID,
			subID: validSubID, competence: "2026-06", operation: "delete",
			version: 2, amountCents: 0, occurredAt: now,
		},
		{
			name:    "invalid_event_id",
			eventID: "x", userID: validUserID, source: "kiwify", extID: validExtID,
			subID: validSubID, competence: "2026-06", operation: "create",
			version: 1, amountCents: 100, occurredAt: now,
			wantErrs: []error{commands.ErrCommandInvalidEventID},
		},
		{
			name:    "invalid_user_id",
			eventID: validEventID, userID: "x", source: "kiwify", extID: validExtID,
			subID: validSubID, competence: "2026-06", operation: "create",
			version: 1, amountCents: 100, occurredAt: now,
			wantErrs: []error{commands.ErrCommandInvalidUserID},
		},
		{
			name:    "invalid_source",
			eventID: validEventID, userID: validUserID, source: "", extID: validExtID,
			subID: validSubID, competence: "2026-06", operation: "create",
			version: 1, amountCents: 100, occurredAt: now,
			wantErrs: []error{commands.ErrCommandInvalidSource},
		},
		{
			name:    "invalid_ext_id",
			eventID: validEventID, userID: validUserID, source: "kiwify", extID: "bad",
			subID: validSubID, competence: "2026-06", operation: "create",
			version: 1, amountCents: 100, occurredAt: now,
			wantErrs: []error{commands.ErrCommandInvalidExternalID},
		},
		{
			name:    "invalid_operation",
			eventID: validEventID, userID: validUserID, source: "kiwify", extID: validExtID,
			subID: validSubID, competence: "2026-06", operation: "bogus",
			version: 1, amountCents: 100, occurredAt: now,
			wantErrs: []error{commands.ErrCommandInvalidMutationKind},
		},
		{
			name:    "invalid_subcategory",
			eventID: validEventID, userID: validUserID, source: "kiwify", extID: validExtID,
			subID: "x", competence: "2026-06", operation: "create",
			version: 1, amountCents: 100, occurredAt: now,
			wantErrs: []error{commands.ErrCommandInvalidSubcategory},
		},
		{
			name:    "invalid_competence",
			eventID: validEventID, userID: validUserID, source: "kiwify", extID: validExtID,
			subID: validSubID, competence: "bad", operation: "create",
			version: 1, amountCents: 100, occurredAt: now,
			wantErrs: []error{commands.ErrCommandInvalidCompetence},
		},
		{
			name:    "invalid_amount_for_non_delete",
			eventID: validEventID, userID: validUserID, source: "kiwify", extID: validExtID,
			subID: validSubID, competence: "2026-06", operation: "update",
			version: 2, amountCents: 0, occurredAt: now,
			wantErrs: []error{commands.ErrCommandInvalidAmount},
		},
		{
			name:    "invalid_occurred_at",
			eventID: validEventID, userID: validUserID, source: "kiwify", extID: validExtID,
			subID: validSubID, competence: "2026-06", operation: "create",
			version: 1, amountCents: 100, occurredAt: time.Time{},
			wantErrs: []error{commands.ErrCommandInvalidOccurredAt},
		},
		{
			name:    "create_version_not_one",
			eventID: validEventID, userID: validUserID, source: "kiwify", extID: validExtID,
			subID: validSubID, competence: "2026-06", operation: "create",
			version: 2, amountCents: 100, occurredAt: now,
			wantErrs: []error{commands.ErrCommandVersionRequired},
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			cmd, err := commands.NewIngestExternalExpenseCommand(tc.eventID, tc.userID, tc.source, tc.extID, tc.subID, tc.competence, tc.operation, tc.version, tc.amountCents, tc.occurredAt)
			if len(tc.wantErrs) == 0 {
				s.Require().NoError(err)
				s.Equal(tc.version, cmd.Version)
				return
			}
			s.Require().Error(err)
			for _, w := range tc.wantErrs {
				s.True(errors.Is(err, w), "expected %v in %v", w, err)
			}
		})
	}
}
