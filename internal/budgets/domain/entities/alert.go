package entities

import (
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type AlertState uint8

const (
	AlertStatePendingDelivery AlertState = iota + 1
	AlertStateDelivered
	AlertStateSuppressedStale
	AlertStateSuppressedRetroactive
	AlertStateRateLimited
)

type Alert struct {
	id                     uuid.UUID
	userID                 uuid.UUID
	competence             valueobjects.Competence
	rootSlug               valueobjects.RootSlug
	threshold              valueobjects.Threshold
	state                  AlertState
	triggeredByCommittedAt time.Time
	spentCents             int64
	plannedCents           int64
	createdAt              time.Time
}

func NewAlert(
	userID uuid.UUID,
	competence valueobjects.Competence,
	rootSlug valueobjects.RootSlug,
	threshold valueobjects.Threshold,
	state AlertState,
	triggeredByCommittedAt time.Time,
	spentCents int64,
	plannedCents int64,
	now time.Time,
) Alert {
	return Alert{
		id:                     uuid.New(),
		userID:                 userID,
		competence:             competence,
		rootSlug:               rootSlug,
		threshold:              threshold,
		state:                  state,
		triggeredByCommittedAt: triggeredByCommittedAt,
		spentCents:             spentCents,
		plannedCents:           plannedCents,
		createdAt:              now,
	}
}

func HydrateAlert(
	id uuid.UUID,
	userID uuid.UUID,
	competence valueobjects.Competence,
	rootSlug valueobjects.RootSlug,
	threshold valueobjects.Threshold,
	state AlertState,
	triggeredByCommittedAt time.Time,
	spentCents int64,
	plannedCents int64,
	createdAt time.Time,
) Alert {
	return Alert{
		id:                     id,
		userID:                 userID,
		competence:             competence,
		rootSlug:               rootSlug,
		threshold:              threshold,
		state:                  state,
		triggeredByCommittedAt: triggeredByCommittedAt,
		spentCents:             spentCents,
		plannedCents:           plannedCents,
		createdAt:              createdAt,
	}
}

func (a Alert) ID() uuid.UUID                       { return a.id }
func (a Alert) UserID() uuid.UUID                   { return a.userID }
func (a Alert) Competence() valueobjects.Competence { return a.competence }
func (a Alert) RootSlug() valueobjects.RootSlug     { return a.rootSlug }
func (a Alert) Threshold() valueobjects.Threshold   { return a.threshold }
func (a Alert) State() AlertState                   { return a.state }
func (a Alert) TriggeredByCommittedAt() time.Time   { return a.triggeredByCommittedAt }
func (a Alert) SpentCents() int64                   { return a.spentCents }
func (a Alert) PlannedCents() int64                 { return a.plannedCents }
func (a Alert) CreatedAt() time.Time                { return a.createdAt }

func (a Alert) IsVisibleToUser() bool {
	return a.state == AlertStatePendingDelivery || a.state == AlertStateDelivered
}
