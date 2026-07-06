package entities

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

var ErrTransactionVersionMismatch = errors.New("transactions: versão esperada não corresponde à versão atual")
var ErrTransactionAlreadyDeleted = errors.New("transactions: lançamento já excluído")

type Transaction struct {
	id                      uuid.UUID
	userID                  valueobjects.UserID
	direction               valueobjects.Direction
	paymentMethod           valueobjects.PaymentMethod
	amount                  valueobjects.Money
	description             valueobjects.Description
	categoryID              valueobjects.CategoryID
	subcategoryID           option.Option[valueobjects.SubcategoryID]
	categoryNameSnapshot    string
	subcategoryNameSnapshot string
	evidence                valueobjects.CategoryWriteEvidence
	refMonth                valueobjects.RefMonth
	occurredAt              time.Time
	cardID                  option.Option[valueobjects.CardID]
	installmentsTotal       option.Option[valueobjects.InstallmentCount]
	billingSnapshot         option.Option[valueobjects.CardBillingSnapshot]
	version                 int64
	deletedAt               *time.Time
	createdAt               time.Time
	updatedAt               time.Time
	originWamid             string
	originItemSeq           int
	originOperation         string
	hasOrigin               bool
}

func NewTransaction(
	id uuid.UUID,
	userID valueobjects.UserID,
	direction valueobjects.Direction,
	paymentMethod valueobjects.PaymentMethod,
	amount valueobjects.Money,
	description valueobjects.Description,
	categoryID valueobjects.CategoryID,
	subcategoryID option.Option[valueobjects.SubcategoryID],
	categoryNameSnapshot string,
	subcategoryNameSnapshot string,
	evidence valueobjects.CategoryWriteEvidence,
	refMonth valueobjects.RefMonth,
	occurredAt time.Time,
	now time.Time,
) Transaction {
	return Transaction{
		id:                      id,
		userID:                  userID,
		direction:               direction,
		paymentMethod:           paymentMethod,
		amount:                  amount,
		description:             description,
		categoryID:              categoryID,
		subcategoryID:           subcategoryID,
		categoryNameSnapshot:    categoryNameSnapshot,
		subcategoryNameSnapshot: subcategoryNameSnapshot,
		evidence:                evidence,
		refMonth:                refMonth,
		occurredAt:              occurredAt,
		version:                 1,
		createdAt:               now,
		updatedAt:               now,
	}
}

func Reconstitute(
	id uuid.UUID,
	userID valueobjects.UserID,
	direction valueobjects.Direction,
	paymentMethod valueobjects.PaymentMethod,
	amount valueobjects.Money,
	description valueobjects.Description,
	categoryID valueobjects.CategoryID,
	subcategoryID option.Option[valueobjects.SubcategoryID],
	categoryNameSnapshot string,
	subcategoryNameSnapshot string,
	evidence valueobjects.CategoryWriteEvidence,
	refMonth valueobjects.RefMonth,
	occurredAt time.Time,
	cardID option.Option[valueobjects.CardID],
	installmentsTotal option.Option[valueobjects.InstallmentCount],
	billingSnapshot option.Option[valueobjects.CardBillingSnapshot],
	version int64,
	deletedAt *time.Time,
	createdAt time.Time,
	updatedAt time.Time,
) Transaction {
	return Transaction{
		id:                      id,
		userID:                  userID,
		direction:               direction,
		paymentMethod:           paymentMethod,
		amount:                  amount,
		description:             description,
		categoryID:              categoryID,
		subcategoryID:           subcategoryID,
		categoryNameSnapshot:    categoryNameSnapshot,
		subcategoryNameSnapshot: subcategoryNameSnapshot,
		evidence:                evidence,
		refMonth:                refMonth,
		occurredAt:              occurredAt,
		cardID:                  cardID,
		installmentsTotal:       installmentsTotal,
		billingSnapshot:         billingSnapshot,
		version:                 version,
		deletedAt:               deletedAt,
		createdAt:               createdAt,
		updatedAt:               updatedAt,
	}
}

func (t *Transaction) ID() uuid.UUID                             { return t.id }
func (t *Transaction) UserID() valueobjects.UserID               { return t.userID }
func (t *Transaction) Direction() valueobjects.Direction         { return t.direction }
func (t *Transaction) PaymentMethod() valueobjects.PaymentMethod { return t.paymentMethod }
func (t *Transaction) Amount() valueobjects.Money                { return t.amount }
func (t *Transaction) Description() valueobjects.Description     { return t.description }
func (t *Transaction) CategoryID() valueobjects.CategoryID       { return t.categoryID }
func (t *Transaction) SubcategoryID() option.Option[valueobjects.SubcategoryID] {
	return t.subcategoryID
}
func (t *Transaction) CategoryNameSnapshot() string                 { return t.categoryNameSnapshot }
func (t *Transaction) SubcategoryNameSnapshot() string              { return t.subcategoryNameSnapshot }
func (t *Transaction) Evidence() valueobjects.CategoryWriteEvidence { return t.evidence }
func (t *Transaction) RefMonth() valueobjects.RefMonth              { return t.refMonth }
func (t *Transaction) OccurredAt() time.Time                        { return t.occurredAt }
func (t *Transaction) CardID() option.Option[valueobjects.CardID] {
	return t.cardID
}
func (t *Transaction) InstallmentsTotal() option.Option[valueobjects.InstallmentCount] {
	return t.installmentsTotal
}
func (t *Transaction) BillingSnapshot() option.Option[valueobjects.CardBillingSnapshot] {
	return t.billingSnapshot
}
func (t *Transaction) Version() int64        { return t.version }
func (t *Transaction) DeletedAt() *time.Time { return t.deletedAt }
func (t *Transaction) CreatedAt() time.Time  { return t.createdAt }
func (t *Transaction) UpdatedAt() time.Time  { return t.updatedAt }

func (t *Transaction) Update(
	direction valueobjects.Direction,
	paymentMethod valueobjects.PaymentMethod,
	amount valueobjects.Money,
	description valueobjects.Description,
	categoryID valueobjects.CategoryID,
	subcategoryID option.Option[valueobjects.SubcategoryID],
	categoryNameSnapshot string,
	subcategoryNameSnapshot string,
	evidence valueobjects.CategoryWriteEvidence,
	refMonth valueobjects.RefMonth,
	occurredAt time.Time,
	now time.Time,
) {
	t.direction = direction
	t.paymentMethod = paymentMethod
	t.amount = amount
	t.description = description
	t.categoryID = categoryID
	t.subcategoryID = subcategoryID
	t.categoryNameSnapshot = categoryNameSnapshot
	t.subcategoryNameSnapshot = subcategoryNameSnapshot
	t.evidence = evidence
	t.refMonth = refMonth
	t.occurredAt = occurredAt
	t.version++
	t.updatedAt = now
}

func (t *Transaction) SetCardBilling(
	cardID valueobjects.CardID,
	installmentsTotal valueobjects.InstallmentCount,
	billingSnapshot valueobjects.CardBillingSnapshot,
) {
	t.cardID = option.Some(cardID)
	t.installmentsTotal = option.Some(installmentsTotal)
	t.billingSnapshot = option.Some(billingSnapshot)
}

func (t *Transaction) SetCategorySnapshots(categoryName, subcategoryName string) {
	t.categoryNameSnapshot = categoryName
	t.subcategoryNameSnapshot = subcategoryName
}

func (t *Transaction) SetOrigin(wamid string, itemSeq int, operation string) {
	t.originWamid = wamid
	t.originItemSeq = itemSeq
	t.originOperation = operation
	t.hasOrigin = true
}

func (t *Transaction) OriginWamid() string     { return t.originWamid }
func (t *Transaction) OriginItemSeq() int      { return t.originItemSeq }
func (t *Transaction) OriginOperation() string { return t.originOperation }
func (t *Transaction) HasOrigin() bool         { return t.hasOrigin }

func (t *Transaction) SoftDelete(now time.Time) error {
	if t.deletedAt != nil {
		return ErrTransactionAlreadyDeleted
	}
	t.deletedAt = &now
	t.version++
	t.updatedAt = now
	return nil
}
