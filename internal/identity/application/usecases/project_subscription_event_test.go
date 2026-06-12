package usecases

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
)

func TestDecideEntitlementPlan_Pending(t *testing.T) {
	t.Parallel()

	projection := interfaces.SubscriptionProjectionRecord{
		SubscriptionID: "sub-1",
		FunnelToken:    "tok-1",
		Status:         "active",
		PeriodEnd:      time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		UserID:         "",
	}

	plan, err := decideEntitlementPlan(projection)
	require.NoError(t, err)
	pending, ok := plan.(PendingEntitlement)
	require.True(t, ok)
	require.Equal(t, "sub-1", pending.SubscriptionID)
	require.Equal(t, "tok-1", pending.FunnelToken)
	require.NotEmpty(t, pending.PayloadRaw)

	var decoded interfaces.SubscriptionProjectionRecord
	require.NoError(t, json.Unmarshal(pending.PayloadRaw, &decoded))
	require.Equal(t, projection.SubscriptionID, decoded.SubscriptionID)
	require.Equal(t, projection.FunnelToken, decoded.FunnelToken)
}

func TestDecideEntitlementPlan_Active(t *testing.T) {
	t.Parallel()

	projection := interfaces.SubscriptionProjectionRecord{
		SubscriptionID: "sub-2",
		FunnelToken:    "tok-2",
		Status:         "active",
		PeriodEnd:      time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		GraceEnd:       time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC),
		UserID:         "user-123",
	}

	plan, err := decideEntitlementPlan(projection)
	require.NoError(t, err)
	committed, ok := plan.(CommittedEntitlement)
	require.True(t, ok)
	require.Equal(t, "user-123", committed.Record.UserID)
	require.Equal(t, "sub-2", committed.Record.SubscriptionID)
	require.Equal(t, "active", committed.Record.Status)
	require.Equal(t, projection.PeriodEnd, committed.Record.PeriodEnd)
	require.Equal(t, projection.GraceEnd, committed.Record.GraceEnd)
}

func TestExtractSubscriptionRef(t *testing.T) {
	t.Parallel()

	raw, err := json.Marshal(subscriptionRefPayload{SubscriptionID: "sub-9"})
	require.NoError(t, err)

	id, err := extractSubscriptionRef(raw, eventTypeActivated)
	require.NoError(t, err)
	require.Equal(t, "sub-9", id)

	_, err = extractSubscriptionRef([]byte("not-json"), eventTypeActivated)
	require.Error(t, err)
	require.Contains(t, err.Error(), eventTypeActivated)
	require.Contains(t, err.Error(), "unmarshal")
}
