package entities

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

var ErrRecurringTemplateAlreadyDeleted = errors.New("transactions: template recorrente já excluído")

type RecurringTemplate struct {
	id                      uuid.UUID
	userID                  valueobjects.UserID
	direction               valueobjects.Direction
	paymentMethod           valueobjects.PaymentMethod
	cardID                  option.Option[valueobjects.CardID]
	amount                  valueobjects.Money
	description             valueobjects.Description
	categoryID              valueobjects.CategoryID
	subcategoryID           option.Option[valueobjects.SubcategoryID]
	categoryNameSnapshot    string
	subcategoryNameSnapshot string
	frequency               valueobjects.Frequency
	dayOfMonth              valueobjects.DayOfMonth
	installmentsTotal       valueobjects.InstallmentCount
	startedAt               time.Time
	endedAt                 option.Option[time.Time]
	version                 int64
	deletedAt               *time.Time
	createdAt               time.Time
	updatedAt               time.Time
}

func NewRecurringTemplate(
	id uuid.UUID,
	userID valueobjects.UserID,
	direction valueobjects.Direction,
	paymentMethod valueobjects.PaymentMethod,
	cardID option.Option[valueobjects.CardID],
	amount valueobjects.Money,
	description valueobjects.Description,
	categoryID valueobjects.CategoryID,
	subcategoryID option.Option[valueobjects.SubcategoryID],
	categoryNameSnapshot string,
	subcategoryNameSnapshot string,
	frequency valueobjects.Frequency,
	dayOfMonth valueobjects.DayOfMonth,
	installmentsTotal valueobjects.InstallmentCount,
	startedAt time.Time,
	endedAt option.Option[time.Time],
	now time.Time,
) RecurringTemplate {
	return RecurringTemplate{
		id:                      id,
		userID:                  userID,
		direction:               direction,
		paymentMethod:           paymentMethod,
		cardID:                  cardID,
		amount:                  amount,
		description:             description,
		categoryID:              categoryID,
		subcategoryID:           subcategoryID,
		categoryNameSnapshot:    categoryNameSnapshot,
		subcategoryNameSnapshot: subcategoryNameSnapshot,
		frequency:               frequency,
		dayOfMonth:              dayOfMonth,
		installmentsTotal:       installmentsTotal,
		startedAt:               startedAt,
		endedAt:                 endedAt,
		version:                 1,
		createdAt:               now,
		updatedAt:               now,
	}
}

func (r *RecurringTemplate) ID() uuid.UUID                              { return r.id }
func (r *RecurringTemplate) UserID() valueobjects.UserID                { return r.userID }
func (r *RecurringTemplate) Direction() valueobjects.Direction          { return r.direction }
func (r *RecurringTemplate) PaymentMethod() valueobjects.PaymentMethod  { return r.paymentMethod }
func (r *RecurringTemplate) CardID() option.Option[valueobjects.CardID] { return r.cardID }
func (r *RecurringTemplate) Amount() valueobjects.Money                 { return r.amount }
func (r *RecurringTemplate) Description() valueobjects.Description      { return r.description }
func (r *RecurringTemplate) CategoryID() valueobjects.CategoryID        { return r.categoryID }
func (r *RecurringTemplate) SubcategoryID() option.Option[valueobjects.SubcategoryID] {
	return r.subcategoryID
}
func (r *RecurringTemplate) CategoryNameSnapshot() string        { return r.categoryNameSnapshot }
func (r *RecurringTemplate) SubcategoryNameSnapshot() string     { return r.subcategoryNameSnapshot }
func (r *RecurringTemplate) Frequency() valueobjects.Frequency   { return r.frequency }
func (r *RecurringTemplate) DayOfMonth() valueobjects.DayOfMonth { return r.dayOfMonth }
func (r *RecurringTemplate) InstallmentsTotal() valueobjects.InstallmentCount {
	return r.installmentsTotal
}
func (r *RecurringTemplate) StartedAt() time.Time              { return r.startedAt }
func (r *RecurringTemplate) EndedAt() option.Option[time.Time] { return r.endedAt }
func (r *RecurringTemplate) Version() int64                    { return r.version }
func (r *RecurringTemplate) DeletedAt() *time.Time             { return r.deletedAt }
func (r *RecurringTemplate) CreatedAt() time.Time              { return r.createdAt }
func (r *RecurringTemplate) UpdatedAt() time.Time              { return r.updatedAt }

func (r *RecurringTemplate) Update(
	direction valueobjects.Direction,
	paymentMethod valueobjects.PaymentMethod,
	cardID option.Option[valueobjects.CardID],
	amount valueobjects.Money,
	description valueobjects.Description,
	categoryID valueobjects.CategoryID,
	subcategoryID option.Option[valueobjects.SubcategoryID],
	categoryNameSnapshot string,
	subcategoryNameSnapshot string,
	frequency valueobjects.Frequency,
	dayOfMonth valueobjects.DayOfMonth,
	installmentsTotal valueobjects.InstallmentCount,
	startedAt time.Time,
	endedAt option.Option[time.Time],
	now time.Time,
) {
	r.direction = direction
	r.paymentMethod = paymentMethod
	r.cardID = cardID
	r.amount = amount
	r.description = description
	r.categoryID = categoryID
	r.subcategoryID = subcategoryID
	r.categoryNameSnapshot = categoryNameSnapshot
	r.subcategoryNameSnapshot = subcategoryNameSnapshot
	r.frequency = frequency
	r.dayOfMonth = dayOfMonth
	r.installmentsTotal = installmentsTotal
	r.startedAt = startedAt
	r.endedAt = endedAt
	r.version++
	r.updatedAt = now
}

func (r *RecurringTemplate) SetCategorySnapshots(categoryName, subcategoryName string) {
	r.categoryNameSnapshot = categoryName
	r.subcategoryNameSnapshot = subcategoryName
}

func (r *RecurringTemplate) SoftDelete(now time.Time) error {
	if r.deletedAt != nil {
		return ErrRecurringTemplateAlreadyDeleted
	}
	r.deletedAt = &now
	r.version++
	r.updatedAt = now
	return nil
}

func ReconstituteRecurringTemplate(
	id uuid.UUID,
	userID valueobjects.UserID,
	direction valueobjects.Direction,
	paymentMethod valueobjects.PaymentMethod,
	cardID option.Option[valueobjects.CardID],
	amount valueobjects.Money,
	description valueobjects.Description,
	categoryID valueobjects.CategoryID,
	subcategoryID option.Option[valueobjects.SubcategoryID],
	categoryNameSnapshot string,
	subcategoryNameSnapshot string,
	frequency valueobjects.Frequency,
	dayOfMonth valueobjects.DayOfMonth,
	installmentsTotal valueobjects.InstallmentCount,
	startedAt time.Time,
	endedAt option.Option[time.Time],
	version int64,
	deletedAt *time.Time,
	createdAt time.Time,
	updatedAt time.Time,
) RecurringTemplate {
	return RecurringTemplate{
		id:                      id,
		userID:                  userID,
		direction:               direction,
		paymentMethod:           paymentMethod,
		cardID:                  cardID,
		amount:                  amount,
		description:             description,
		categoryID:              categoryID,
		subcategoryID:           subcategoryID,
		categoryNameSnapshot:    categoryNameSnapshot,
		subcategoryNameSnapshot: subcategoryNameSnapshot,
		frequency:               frequency,
		dayOfMonth:              dayOfMonth,
		installmentsTotal:       installmentsTotal,
		startedAt:               startedAt,
		endedAt:                 endedAt,
		version:                 version,
		deletedAt:               deletedAt,
		createdAt:               createdAt,
		updatedAt:               updatedAt,
	}
}
