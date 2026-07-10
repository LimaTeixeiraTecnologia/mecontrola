package services

import (
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
)

type PrincipalDecision struct {
	UserID      uuid.UUID
	Found       bool
	EventID     uuid.UUID
	EventKind   entities.AuthEventKind
	ResolvePath domain.AuthResolvePath
	OccurredAt  time.Time
}

type PrincipalWorkflow struct{}

func (PrincipalWorkflow) DecidePrincipal(userID uuid.UUID, found bool, resolvePath domain.AuthResolvePath, eventID uuid.UUID, now time.Time) PrincipalDecision {
	if !found || userID == uuid.Nil {
		return PrincipalDecision{
			Found:      false,
			EventID:    eventID,
			EventKind:  entities.AuthEventKindUnknownUser,
			OccurredAt: now,
		}
	}
	return PrincipalDecision{
		UserID:      userID,
		Found:       true,
		EventID:     eventID,
		EventKind:   entities.AuthEventKindPrincipalEstablished,
		ResolvePath: resolvePath,
		OccurredAt:  now,
	}
}
