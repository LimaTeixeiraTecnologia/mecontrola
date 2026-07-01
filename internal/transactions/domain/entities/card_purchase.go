package entities

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

var ErrCardPurchaseAlreadyDeleted = errors.New("transactions: compra já excluída")

type CardPurchase struct {
	id                      uuid.UUID
	userID                  valueobjects.UserID
	cardID                  valueobjects.CardID
	totalAmount             valueobjects.Money
	installmentsTotal       valueobjects.InstallmentCount
	description             valueobjects.Description
	categoryID              valueobjects.CategoryID
	subcategoryID           option.Option[valueobjects.SubcategoryID]
	categoryNameSnapshot    string
	subcategoryNameSnapshot string
	purchasedAt             time.Time
	billingSnapshot         valueobjects.CardBillingSnapshot
	version                 int64
	deletedAt               *time.Time
	createdAt               time.Time
	updatedAt               time.Time
	originWamid             string
	originItemSeq           int
	originOperation         string
	hasOrigin               bool
}

func NewCardPurchase(
	id uuid.UUID,
	userID valueobjects.UserID,
	cardID valueobjects.CardID,
	totalAmount valueobjects.Money,
	installmentsTotal valueobjects.InstallmentCount,
	description valueobjects.Description,
	categoryID valueobjects.CategoryID,
	subcategoryID option.Option[valueobjects.SubcategoryID],
	categoryNameSnapshot string,
	subcategoryNameSnapshot string,
	purchasedAt time.Time,
	billingSnapshot valueobjects.CardBillingSnapshot,
	now time.Time,
) CardPurchase {
	return CardPurchase{
		id:                      id,
		userID:                  userID,
		cardID:                  cardID,
		totalAmount:             totalAmount,
		installmentsTotal:       installmentsTotal,
		description:             description,
		categoryID:              categoryID,
		subcategoryID:           subcategoryID,
		categoryNameSnapshot:    categoryNameSnapshot,
		subcategoryNameSnapshot: subcategoryNameSnapshot,
		purchasedAt:             purchasedAt,
		billingSnapshot:         billingSnapshot,
		version:                 1,
		createdAt:               now,
		updatedAt:               now,
	}
}

func (p *CardPurchase) ID() uuid.UUID                                    { return p.id }
func (p *CardPurchase) UserID() valueobjects.UserID                      { return p.userID }
func (p *CardPurchase) CardID() valueobjects.CardID                      { return p.cardID }
func (p *CardPurchase) TotalAmount() valueobjects.Money                  { return p.totalAmount }
func (p *CardPurchase) InstallmentsTotal() valueobjects.InstallmentCount { return p.installmentsTotal }
func (p *CardPurchase) Description() valueobjects.Description            { return p.description }
func (p *CardPurchase) CategoryID() valueobjects.CategoryID              { return p.categoryID }
func (p *CardPurchase) SubcategoryID() option.Option[valueobjects.SubcategoryID] {
	return p.subcategoryID
}
func (p *CardPurchase) CategoryNameSnapshot() string                      { return p.categoryNameSnapshot }
func (p *CardPurchase) SubcategoryNameSnapshot() string                   { return p.subcategoryNameSnapshot }
func (p *CardPurchase) PurchasedAt() time.Time                            { return p.purchasedAt }
func (p *CardPurchase) BillingSnapshot() valueobjects.CardBillingSnapshot { return p.billingSnapshot }
func (p *CardPurchase) Version() int64                                    { return p.version }
func (p *CardPurchase) DeletedAt() *time.Time                             { return p.deletedAt }
func (p *CardPurchase) CreatedAt() time.Time                              { return p.createdAt }
func (p *CardPurchase) UpdatedAt() time.Time                              { return p.updatedAt }

func (p *CardPurchase) Update(
	totalAmount valueobjects.Money,
	installmentsTotal valueobjects.InstallmentCount,
	description valueobjects.Description,
	categoryID valueobjects.CategoryID,
	subcategoryID option.Option[valueobjects.SubcategoryID],
	categoryNameSnapshot string,
	subcategoryNameSnapshot string,
	purchasedAt time.Time,
	now time.Time,
) {
	p.totalAmount = totalAmount
	p.installmentsTotal = installmentsTotal
	p.description = description
	p.categoryID = categoryID
	p.subcategoryID = subcategoryID
	p.categoryNameSnapshot = categoryNameSnapshot
	p.subcategoryNameSnapshot = subcategoryNameSnapshot
	p.purchasedAt = purchasedAt
	p.version++
	p.updatedAt = now
}

func (p *CardPurchase) SetCategoryNameSnapshot(name string) {
	p.categoryNameSnapshot = name
}

func (p *CardPurchase) SetSubcategoryNameSnapshot(name string) {
	p.subcategoryNameSnapshot = name
}

func (p *CardPurchase) UpdateNameSnapshots(categoryName, subcategoryName string) {
	p.categoryNameSnapshot = categoryName
	p.subcategoryNameSnapshot = subcategoryName
}

func (p *CardPurchase) SetOrigin(wamid string, itemSeq int, operation string) {
	p.originWamid = wamid
	p.originItemSeq = itemSeq
	p.originOperation = operation
	p.hasOrigin = true
}

func (p *CardPurchase) OriginWamid() string     { return p.originWamid }
func (p *CardPurchase) OriginItemSeq() int      { return p.originItemSeq }
func (p *CardPurchase) OriginOperation() string { return p.originOperation }
func (p *CardPurchase) HasOrigin() bool         { return p.hasOrigin }

func (p *CardPurchase) HydrateVersion(v int64, updatedAt time.Time) {
	p.version = v
	p.updatedAt = updatedAt
}

func (p *CardPurchase) SoftDelete(now time.Time) error {
	if p.deletedAt != nil {
		return ErrCardPurchaseAlreadyDeleted
	}
	p.deletedAt = &now
	p.version++
	p.updatedAt = now
	return nil
}
