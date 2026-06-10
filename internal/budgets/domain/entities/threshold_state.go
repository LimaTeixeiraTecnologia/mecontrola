package entities

import (
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type ThresholdKey struct {
	UserID     uuid.UUID
	Competence valueobjects.Competence
	RootSlug   valueobjects.RootSlug
	Threshold  valueobjects.Threshold
}

type ThresholdState struct {
	key                      ThresholdKey
	currentlyCrossed         bool
	version                  int64
	lastCrossedAt            *time.Time
	lastUncrossedAt          *time.Time
	lastEvaluatedCommittedAt *time.Time
}

func NewThresholdState(key ThresholdKey) ThresholdState {
	return ThresholdState{
		key: key,
	}
}

func HydrateThresholdState(
	key ThresholdKey,
	currentlyCrossed bool,
	version int64,
	lastCrossedAt *time.Time,
	lastUncrossedAt *time.Time,
	lastEvaluatedCommittedAt *time.Time,
) ThresholdState {
	return ThresholdState{
		key:                      key,
		currentlyCrossed:         currentlyCrossed,
		version:                  version,
		lastCrossedAt:            lastCrossedAt,
		lastUncrossedAt:          lastUncrossedAt,
		lastEvaluatedCommittedAt: lastEvaluatedCommittedAt,
	}
}

func (ts ThresholdState) Key() ThresholdKey                    { return ts.key }
func (ts ThresholdState) CurrentlyCrossed() bool               { return ts.currentlyCrossed }
func (ts ThresholdState) Version() int64                       { return ts.version }
func (ts ThresholdState) LastCrossedAt() *time.Time            { return ts.lastCrossedAt }
func (ts ThresholdState) LastUncrossedAt() *time.Time          { return ts.lastUncrossedAt }
func (ts ThresholdState) LastEvaluatedCommittedAt() *time.Time { return ts.lastEvaluatedCommittedAt }
