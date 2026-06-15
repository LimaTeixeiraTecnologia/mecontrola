package services

import (
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

type IdentityResolutionKind int

const (
	IdentityResolutionResolved IdentityResolutionKind = iota + 1
	IdentityResolutionUnknown
	IdentityResolutionUnlinked
	IdentityResolutionMismatch
)

func (k IdentityResolutionKind) String() string {
	switch k {
	case IdentityResolutionResolved:
		return "resolved"
	case IdentityResolutionUnknown:
		return "unknown"
	case IdentityResolutionUnlinked:
		return "unlinked"
	case IdentityResolutionMismatch:
		return "mismatch"
	default:
		return "invalid"
	}
}

type IdentityResolutionDecision struct {
	Kind       IdentityResolutionKind
	UserID     uuid.UUID
	Channel    valueobjects.Channel
	ExternalID valueobjects.ExternalID
	EventID    uuid.UUID
	OccurredAt time.Time
}

type IdentityResolutionWorkflow struct{}

func (w IdentityResolutionWorkflow) DecideResolve(
	candidate entities.UserIdentity,
	found bool,
	expectedChannel valueobjects.Channel,
	expectedExternalID valueobjects.ExternalID,
	eventID uuid.UUID,
	now time.Time,
) IdentityResolutionDecision {
	base := IdentityResolutionDecision{
		Channel:    expectedChannel,
		ExternalID: expectedExternalID,
		EventID:    eventID,
		OccurredAt: now,
	}

	if !found {
		base.Kind = IdentityResolutionUnknown
		return base
	}

	if !candidate.Channel().Equal(expectedChannel) || !candidate.ExternalID().Equal(expectedExternalID) {
		base.Kind = IdentityResolutionMismatch
		base.UserID = candidate.UserID()
		return base
	}

	if !candidate.IsActive() {
		base.Kind = IdentityResolutionUnlinked
		base.UserID = candidate.UserID()
		return base
	}

	base.Kind = IdentityResolutionResolved
	base.UserID = candidate.UserID()
	return base
}
