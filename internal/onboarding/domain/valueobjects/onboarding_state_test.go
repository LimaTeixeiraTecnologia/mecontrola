package valueobjects_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

func TestOnboardingState_RoundTrip(t *testing.T) {
	t.Parallel()

	cases := []struct {
		raw    string
		parsed valueobjects.OnboardingState
	}{
		{"awaiting_token", valueobjects.OnboardingStateAwaitingToken},
		{"awaiting_income", valueobjects.OnboardingStateAwaitingIncome},
		{"awaiting_card_decision", valueobjects.OnboardingStateAwaitingCardDecision},
		{"awaiting_card_name", valueobjects.OnboardingStateAwaitingCardName},
		{"awaiting_card_limit", valueobjects.OnboardingStateAwaitingCardLimit},
		{"awaiting_card_closing_day", valueobjects.OnboardingStateAwaitingCardClosingDay},
		{"awaiting_card_due_day", valueobjects.OnboardingStateAwaitingCardDueDay},
		{"awaiting_more_cards", valueobjects.OnboardingStateAwaitingMoreCards},
		{"awaiting_split_confirm", valueobjects.OnboardingStateAwaitingSplitConfirm},
		{"active", valueobjects.OnboardingStateActive},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.raw, func(t *testing.T) {
			t.Parallel()
			got, err := valueobjects.ParseOnboardingState(tc.raw)
			require.NoError(t, err)
			require.Equal(t, tc.parsed, got)
			require.Equal(t, tc.raw, got.String())
		})
	}
}

func TestOnboardingState_InvalidParse(t *testing.T) {
	t.Parallel()
	_, err := valueobjects.ParseOnboardingState("nope")
	require.Error(t, err)
	require.True(t, errors.Is(err, valueobjects.ErrOnboardingStateInvalid))
}

func TestOnboardingState_IsTerminal(t *testing.T) {
	t.Parallel()
	require.True(t, valueobjects.OnboardingStateActive.IsTerminal())
	require.False(t, valueobjects.OnboardingStateAwaitingToken.IsTerminal())
	require.False(t, valueobjects.OnboardingStateAwaitingSplitConfirm.IsTerminal())
}
