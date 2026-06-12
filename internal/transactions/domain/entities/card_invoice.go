package entities

import (
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type CardInvoice struct {
	id              uuid.UUID
	userID          valueobjects.UserID
	cardID          valueobjects.CardID
	refMonth        valueobjects.RefMonth
	closingAt       time.Time
	dueAt           time.Time
	itemsTotalCents int64
	version         int64
	createdAt       time.Time
	updatedAt       time.Time
}

func NewCardInvoice(
	id uuid.UUID,
	userID valueobjects.UserID,
	cardID valueobjects.CardID,
	refMonth valueobjects.RefMonth,
	closingAt time.Time,
	dueAt time.Time,
	now time.Time,
) CardInvoice {
	return CardInvoice{
		id:        id,
		userID:    userID,
		cardID:    cardID,
		refMonth:  refMonth,
		closingAt: closingAt,
		dueAt:     dueAt,
		version:   1,
		createdAt: now,
		updatedAt: now,
	}
}

func (i *CardInvoice) ID() uuid.UUID                   { return i.id }
func (i *CardInvoice) UserID() valueobjects.UserID     { return i.userID }
func (i *CardInvoice) CardID() valueobjects.CardID     { return i.cardID }
func (i *CardInvoice) RefMonth() valueobjects.RefMonth { return i.refMonth }
func (i *CardInvoice) ClosingAt() time.Time            { return i.closingAt }
func (i *CardInvoice) DueAt() time.Time                { return i.dueAt }
func (i *CardInvoice) ItemsTotalCents() int64          { return i.itemsTotalCents }
func (i *CardInvoice) Version() int64                  { return i.version }
func (i *CardInvoice) CreatedAt() time.Time            { return i.createdAt }
func (i *CardInvoice) UpdatedAt() time.Time            { return i.updatedAt }

func (i *CardInvoice) HydrateVersion(v int64, itemsTotalCents int64, updatedAt time.Time) {
	i.version = v
	i.itemsTotalCents = itemsTotalCents
	i.updatedAt = updatedAt
}
