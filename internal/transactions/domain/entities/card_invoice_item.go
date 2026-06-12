package entities

import (
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type CardInvoiceItem struct {
	id               uuid.UUID
	invoiceID        uuid.UUID
	purchaseID       uuid.UUID
	userID           valueobjects.UserID
	refMonth         valueobjects.RefMonth
	installmentIndex int
	amount           valueobjects.Money
	deletedAt        *time.Time
	createdAt        time.Time
	updatedAt        time.Time
}

func NewCardInvoiceItem(
	id uuid.UUID,
	invoiceID uuid.UUID,
	purchaseID uuid.UUID,
	userID valueobjects.UserID,
	refMonth valueobjects.RefMonth,
	installmentIndex int,
	amount valueobjects.Money,
	now time.Time,
) CardInvoiceItem {
	return CardInvoiceItem{
		id:               id,
		invoiceID:        invoiceID,
		purchaseID:       purchaseID,
		userID:           userID,
		refMonth:         refMonth,
		installmentIndex: installmentIndex,
		amount:           amount,
		createdAt:        now,
		updatedAt:        now,
	}
}

func (i *CardInvoiceItem) ID() uuid.UUID                   { return i.id }
func (i *CardInvoiceItem) InvoiceID() uuid.UUID            { return i.invoiceID }
func (i *CardInvoiceItem) PurchaseID() uuid.UUID           { return i.purchaseID }
func (i *CardInvoiceItem) UserID() valueobjects.UserID     { return i.userID }
func (i *CardInvoiceItem) RefMonth() valueobjects.RefMonth { return i.refMonth }
func (i *CardInvoiceItem) InstallmentIndex() int           { return i.installmentIndex }
func (i *CardInvoiceItem) Amount() valueobjects.Money      { return i.amount }
func (i *CardInvoiceItem) DeletedAt() *time.Time           { return i.deletedAt }
func (i *CardInvoiceItem) CreatedAt() time.Time            { return i.createdAt }
func (i *CardInvoiceItem) UpdatedAt() time.Time            { return i.updatedAt }
