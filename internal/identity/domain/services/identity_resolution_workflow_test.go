package services_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

func buildIdentity(t *testing.T, channelRaw, externalIDRaw string) (entities.UserIdentity, valueobjects.Channel, valueobjects.ExternalID) {
	t.Helper()
	channel, err := valueobjects.NewChannel(channelRaw)
	require.NoError(t, err)
	externalID, err := valueobjects.NewExternalID(channel, externalIDRaw)
	require.NoError(t, err)
	identity, err := entities.NewUserIdentity(uuid.New(), uuid.New(), channel, externalID, time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	return identity, channel, externalID
}

func TestIdentityResolutionWorkflow_DecideResolve(t *testing.T) {
	sut := services.IdentityResolutionWorkflow{}
	eventID := uuid.New()
	now := time.Date(2026, 6, 13, 11, 0, 0, 0, time.UTC)

	t.Run("resolved when active and matches", func(t *testing.T) {
		identity, channel, externalID := buildIdentity(t, "whatsapp", "+5511987654321")
		decision := sut.DecideResolve(identity, true, channel, externalID, eventID, now)

		assert.Equal(t, services.IdentityResolutionResolved, decision.Kind)
		assert.Equal(t, identity.UserID(), decision.UserID)
		assert.Equal(t, eventID, decision.EventID)
		assert.Equal(t, now, decision.OccurredAt)
		assert.Equal(t, "resolved", decision.Kind.String())
	})

	t.Run("unknown when not found", func(t *testing.T) {
		_, channel, externalID := buildIdentity(t, "whatsapp", "+5511999990000")
		decision := sut.DecideResolve(entities.UserIdentity{}, false, channel, externalID, eventID, now)

		assert.Equal(t, services.IdentityResolutionUnknown, decision.Kind)
		assert.Equal(t, uuid.Nil, decision.UserID)
	})

	t.Run("unlinked when identity present but inactive", func(t *testing.T) {
		identity, channel, externalID := buildIdentity(t, "whatsapp", "+5511999990001")
		require.NoError(t, identity.Unlink(now.Add(-time.Hour)))

		decision := sut.DecideResolve(identity, true, channel, externalID, eventID, now)

		assert.Equal(t, services.IdentityResolutionUnlinked, decision.Kind)
		assert.Equal(t, identity.UserID(), decision.UserID)
	})
}

func TestIdentityResolutionKind_String(t *testing.T) {
	cases := map[services.IdentityResolutionKind]string{
		services.IdentityResolutionResolved: "resolved",
		services.IdentityResolutionUnknown:  "unknown",
		services.IdentityResolutionUnlinked: "unlinked",
		services.IdentityResolutionMismatch: "mismatch",
		services.IdentityResolutionKind(0):  "invalid",
		services.IdentityResolutionKind(99): "invalid",
	}
	for k, want := range cases {
		assert.Equal(t, want, k.String())
	}
}
