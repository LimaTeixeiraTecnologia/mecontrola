package entities

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

var ErrUserIdentityUserIDRequired = errors.New("identity: user_id is required for user_identity")

var ErrUserIdentityChannelRequired = errors.New("identity: channel is required for user_identity")

var ErrUserIdentityExternalIDRequired = errors.New("identity: external_id is required for user_identity")

var ErrUserIdentityAlreadyUnlinked = errors.New("identity: user_identity already unlinked")

type UserIdentity struct {
	id         uuid.UUID
	userID     uuid.UUID
	channel    valueobjects.Channel
	externalID valueobjects.ExternalID
	verifiedAt time.Time
	createdAt  time.Time
	unlinkedAt time.Time
}

func NewUserIdentity(
	id uuid.UUID,
	userID uuid.UUID,
	channel valueobjects.Channel,
	externalID valueobjects.ExternalID,
	now time.Time,
) (UserIdentity, error) {
	if id == uuid.Nil {
		return UserIdentity{}, fmt.Errorf("identity: %w", errors.New("id is required for user_identity"))
	}
	if userID == uuid.Nil {
		return UserIdentity{}, ErrUserIdentityUserIDRequired
	}
	if channel.IsZero() {
		return UserIdentity{}, ErrUserIdentityChannelRequired
	}
	if externalID.IsZero() {
		return UserIdentity{}, ErrUserIdentityExternalIDRequired
	}
	if !externalID.Channel().Equal(channel) {
		return UserIdentity{}, fmt.Errorf("identity: external_id channel %q does not match identity channel %q",
			externalID.Channel().String(), channel.String())
	}
	return UserIdentity{
		id:         id,
		userID:     userID,
		channel:    channel,
		externalID: externalID,
		verifiedAt: now,
		createdAt:  now,
	}, nil
}

func HydrateUserIdentity(
	id uuid.UUID,
	userID uuid.UUID,
	channelRaw string,
	externalIDRaw string,
	verifiedAt time.Time,
	createdAt time.Time,
	unlinkedAt time.Time,
) (UserIdentity, error) {
	channel, err := valueobjects.NewChannel(channelRaw)
	if err != nil {
		return UserIdentity{}, fmt.Errorf("identity: hydrate channel: %w", err)
	}
	externalID, err := valueobjects.NewExternalID(channel, externalIDRaw)
	if err != nil {
		return UserIdentity{}, fmt.Errorf("identity: hydrate external_id: %w", err)
	}
	if id == uuid.Nil {
		return UserIdentity{}, errors.New("identity: hydrate id is nil")
	}
	if userID == uuid.Nil {
		return UserIdentity{}, ErrUserIdentityUserIDRequired
	}
	return UserIdentity{
		id:         id,
		userID:     userID,
		channel:    channel,
		externalID: externalID,
		verifiedAt: verifiedAt,
		createdAt:  createdAt,
		unlinkedAt: unlinkedAt,
	}, nil
}

func (i UserIdentity) ID() uuid.UUID                       { return i.id }
func (i UserIdentity) UserID() uuid.UUID                   { return i.userID }
func (i UserIdentity) Channel() valueobjects.Channel       { return i.channel }
func (i UserIdentity) ExternalID() valueobjects.ExternalID { return i.externalID }
func (i UserIdentity) VerifiedAt() time.Time               { return i.verifiedAt }
func (i UserIdentity) CreatedAt() time.Time                { return i.createdAt }
func (i UserIdentity) UnlinkedAt() time.Time               { return i.unlinkedAt }

func (i UserIdentity) IsActive() bool { return i.unlinkedAt.IsZero() }

func (i *UserIdentity) Unlink(now time.Time) error {
	if !i.unlinkedAt.IsZero() {
		return ErrUserIdentityAlreadyUnlinked
	}
	i.unlinkedAt = now
	return nil
}
