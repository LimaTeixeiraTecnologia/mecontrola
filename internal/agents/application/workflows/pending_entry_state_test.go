package workflows

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPendingStatus_RoundTrip(t *testing.T) {
	cases := []struct {
		status PendingStatus
		str    string
	}{
		{PendingStatusActive, "active"},
		{PendingStatusCompleted, "completed"},
		{PendingStatusCancelled, "cancelled"},
		{PendingStatusExpired, "expired"},
		{PendingStatusReplaced, "replaced"},
	}
	for _, c := range cases {
		t.Run(c.str, func(t *testing.T) {
			require.Equal(t, c.str, c.status.String())
			parsed, err := ParsePendingStatus(c.str)
			require.NoError(t, err)
			require.Equal(t, c.status, parsed)
			require.True(t, c.status.IsValid())
		})
	}
}

func TestPendingStatus_Invalid(t *testing.T) {
	_, err := ParsePendingStatus("invalid_status")
	require.Error(t, err)
	require.ErrorIs(t, err, errInvalidPendingStatus)

	var zero PendingStatus
	require.False(t, zero.IsValid())
	require.Equal(t, "unknown", zero.String())
}

func TestAwaitingSlot_RoundTrip(t *testing.T) {
	cases := []struct {
		slot AwaitingSlot
		str  string
	}{
		{AwaitingSlotCategory, "category"},
		{AwaitingSlotPaymentMethod, "payment_method"},
		{AwaitingSlotCard, "card"},
		{AwaitingSlotDate, "date"},
		{AwaitingSlotConfirmation, "confirmation"},
		{AwaitingSlotCorrection, "correction"},
	}
	for _, c := range cases {
		t.Run(c.str, func(t *testing.T) {
			require.Equal(t, c.str, c.slot.String())
			parsed, err := ParseAwaitingSlot(c.str)
			require.NoError(t, err)
			require.Equal(t, c.slot, parsed)
			require.True(t, c.slot.IsValid())
		})
	}
}

func TestAwaitingSlot_Invalid(t *testing.T) {
	_, err := ParseAwaitingSlot("unknown_slot")
	require.Error(t, err)
	require.ErrorIs(t, err, errInvalidAwaitingSlot)

	var zero AwaitingSlot
	require.False(t, zero.IsValid())
	require.Equal(t, "unknown", zero.String())
}

func TestPendingOperationKind_RoundTrip(t *testing.T) {
	cases := []struct {
		kind PendingOperationKind
		str  string
	}{
		{PendingOpRegisterExpense, "register_expense"},
		{PendingOpRegisterIncome, "register_income"},
		{PendingOpEditEntry, "edit_entry"},
		{PendingOpCreateRecurrence, "create_recurrence"},
	}
	for _, c := range cases {
		t.Run(c.str, func(t *testing.T) {
			require.Equal(t, c.str, c.kind.String())
			parsed, err := ParsePendingOperationKind(c.str)
			require.NoError(t, err)
			require.Equal(t, c.kind, parsed)
			require.True(t, c.kind.IsValid())
		})
	}
}

func TestPendingOperationKind_Invalid(t *testing.T) {
	_, err := ParsePendingOperationKind("bad_op")
	require.Error(t, err)
	require.ErrorIs(t, err, errInvalidPendingOperationKind)

	var zero PendingOperationKind
	require.False(t, zero.IsValid())
	require.Equal(t, "unknown", zero.String())
}
