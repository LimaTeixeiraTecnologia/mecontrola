package entities_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

func mustChannel(t *testing.T, raw string) valueobjects.Channel {
	t.Helper()
	c, err := valueobjects.NewChannel(raw)
	require.NoError(t, err)
	return c
}

func mustExternalID(t *testing.T, channel valueobjects.Channel, raw string) valueobjects.ExternalID {
	t.Helper()
	e, err := valueobjects.NewExternalID(channel, raw)
	require.NoError(t, err)
	return e
}

func TestNewUserIdentity_Success(t *testing.T) {
	now := time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC)
	id := uuid.New()
	userID := uuid.New()
	channel := mustChannel(t, "telegram")
	externalID := mustExternalID(t, channel, "987654321")

	identity, err := entities.NewUserIdentity(id, userID, channel, externalID, now)
	require.NoError(t, err)

	assert.Equal(t, id, identity.ID())
	assert.Equal(t, userID, identity.UserID())
	assert.True(t, identity.Channel().Equal(channel))
	assert.True(t, identity.ExternalID().Equal(externalID))
	assert.Equal(t, now, identity.VerifiedAt())
	assert.Equal(t, now, identity.CreatedAt())
	assert.True(t, identity.UnlinkedAt().IsZero())
	assert.True(t, identity.IsActive())
}

func TestNewUserIdentity_Errors(t *testing.T) {
	now := time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC)
	wa := mustChannel(t, "whatsapp")
	tg := mustChannel(t, "telegram")
	waExt := mustExternalID(t, wa, "+5511987654321")
	tgExt := mustExternalID(t, tg, "12345")
	validID := uuid.New()
	validUser := uuid.New()

	cases := []struct {
		name       string
		id         uuid.UUID
		userID     uuid.UUID
		channel    valueobjects.Channel
		externalID valueobjects.ExternalID
		wantErr    error
	}{
		{name: "nil id", id: uuid.Nil, userID: validUser, channel: wa, externalID: waExt},
		{name: "nil user id", id: validID, userID: uuid.Nil, channel: wa, externalID: waExt, wantErr: entities.ErrUserIdentityUserIDRequired},
		{name: "zero channel", id: validID, userID: validUser, channel: valueobjects.Channel{}, externalID: waExt, wantErr: entities.ErrUserIdentityChannelRequired},
		{name: "zero external id", id: validID, userID: validUser, channel: wa, externalID: valueobjects.ExternalID{}, wantErr: entities.ErrUserIdentityExternalIDRequired},
		{name: "channel external mismatch", id: validID, userID: validUser, channel: wa, externalID: tgExt},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := entities.NewUserIdentity(tc.id, tc.userID, tc.channel, tc.externalID, now)
			require.Error(t, err)
			if tc.wantErr != nil {
				assert.True(t, errors.Is(err, tc.wantErr), "expected %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestHydrateUserIdentity_Success(t *testing.T) {
	now := time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC)
	id := uuid.New()
	userID := uuid.New()

	identity, err := entities.HydrateUserIdentity(id, userID, "whatsapp", "+5511987654321", now, now, time.Time{})
	require.NoError(t, err)

	assert.Equal(t, id, identity.ID())
	assert.Equal(t, userID, identity.UserID())
	assert.True(t, identity.IsActive())
}

func TestHydrateUserIdentity_Unlinked(t *testing.T) {
	now := time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC)
	unlinkedAt := now.Add(time.Hour)

	identity, err := entities.HydrateUserIdentity(uuid.New(), uuid.New(), "telegram", "555", now, now, unlinkedAt)
	require.NoError(t, err)

	assert.False(t, identity.IsActive())
	assert.Equal(t, unlinkedAt, identity.UnlinkedAt())
}

func TestUserIdentity_Unlink(t *testing.T) {
	now := time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC)
	id := uuid.New()
	userID := uuid.New()
	channel := mustChannel(t, "telegram")
	externalID := mustExternalID(t, channel, "555")

	identity, err := entities.NewUserIdentity(id, userID, channel, externalID, now)
	require.NoError(t, err)

	unlinkAt := now.Add(time.Hour)
	require.NoError(t, identity.Unlink(unlinkAt))
	assert.False(t, identity.IsActive())
	assert.Equal(t, unlinkAt, identity.UnlinkedAt())

	err = identity.Unlink(unlinkAt.Add(time.Hour))
	require.Error(t, err)
	assert.True(t, errors.Is(err, entities.ErrUserIdentityAlreadyUnlinked))
}
