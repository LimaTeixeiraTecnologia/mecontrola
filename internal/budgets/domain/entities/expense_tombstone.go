package entities

import (
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type ExpenseTombstone struct {
	userID                uuid.UUID
	source                valueobjects.ProducerSource
	externalTransactionID valueobjects.ExternalTransactionID
	tombstoneVersion      int64
	deletedAt             time.Time
}

func NewExpenseTombstone(
	userID uuid.UUID,
	source valueobjects.ProducerSource,
	externalTransactionID valueobjects.ExternalTransactionID,
	tombstoneVersion int64,
	deletedAt time.Time,
) ExpenseTombstone {
	return ExpenseTombstone{
		userID:                userID,
		source:                source,
		externalTransactionID: externalTransactionID,
		tombstoneVersion:      tombstoneVersion,
		deletedAt:             deletedAt,
	}
}

func (t ExpenseTombstone) UserID() uuid.UUID                   { return t.userID }
func (t ExpenseTombstone) Source() valueobjects.ProducerSource { return t.source }
func (t ExpenseTombstone) ExternalTransactionID() valueobjects.ExternalTransactionID {
	return t.externalTransactionID
}
func (t ExpenseTombstone) TombstoneVersion() int64 { return t.tombstoneVersion }
func (t ExpenseTombstone) DeletedAt() time.Time    { return t.deletedAt }

func (t ExpenseTombstone) IsPresent() bool {
	return t.tombstoneVersion > 0
}
