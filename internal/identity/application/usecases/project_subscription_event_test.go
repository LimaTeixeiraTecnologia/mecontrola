package usecases

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
)

func TestPlanEntitlementUpsert_Pending(t *testing.T) {
	t.Parallel()

	projection := interfaces.SubscriptionProjectionRecord{
		SubscriptionID: "sub-1",
		FunnelToken:    "tok-1",
		Status:         "active",
		PeriodEnd:      time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		UserID:         "",
	}

	plan, err := planEntitlementUpsert(projection)
	require.NoError(t, err)
	require.True(t, plan.isPending)
	require.NotEmpty(t, plan.pendingRaw)
	require.Equal(t, interfaces.EntitlementRecord{}, plan.record)

	var decoded interfaces.SubscriptionProjectionRecord
	require.NoError(t, json.Unmarshal(plan.pendingRaw, &decoded))
	require.Equal(t, projection.SubscriptionID, decoded.SubscriptionID)
	require.Equal(t, projection.FunnelToken, decoded.FunnelToken)
}

func TestPlanEntitlementUpsert_Active(t *testing.T) {
	t.Parallel()

	projection := interfaces.SubscriptionProjectionRecord{
		SubscriptionID: "sub-2",
		FunnelToken:    "tok-2",
		Status:         "active",
		PeriodEnd:      time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		GraceEnd:       time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC),
		UserID:         "user-123",
	}

	plan, err := planEntitlementUpsert(projection)
	require.NoError(t, err)
	require.False(t, plan.isPending)
	require.Nil(t, plan.pendingRaw)
	require.Equal(t, "user-123", plan.record.UserID)
	require.Equal(t, "sub-2", plan.record.SubscriptionID)
	require.Equal(t, "active", plan.record.Status)
	require.Equal(t, projection.PeriodEnd, plan.record.PeriodEnd)
	require.Equal(t, projection.GraceEnd, plan.record.GraceEnd)
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
