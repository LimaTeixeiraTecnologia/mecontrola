package entities

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

var ErrExpenseVersionMismatch = errors.New("budgets: versão esperada não corresponde à versão atual")

var ErrExpenseAlreadyDeleted = errors.New("budgets: despesa já excluída")

var ErrExpenseInvalidAmount = errors.New("budgets: valor da despesa deve ser inteiro em centavos maior que zero")

type ExpenseIdentity struct {
	UserID                uuid.UUID
	Source                valueobjects.ProducerSource
	ExternalTransactionID valueobjects.ExternalTransactionID
}

type Expense struct {
	id                    uuid.UUID
	userID                uuid.UUID
	source                valueobjects.ProducerSource
	externalTransactionID valueobjects.ExternalTransactionID
	subcategoryID         uuid.UUID
	rootSlug              valueobjects.RootSlug
	competence            valueobjects.Competence
	amountCents           int64
	occurredAt            time.Time
	version               int64
	tombstoneVersion      *int64
	deletedAt             *time.Time
	createdAt             time.Time
	updatedAt             time.Time
}

func NewExpense(
	userID uuid.UUID,
	source valueobjects.ProducerSource,
	externalTransactionID valueobjects.ExternalTransactionID,
	subcategoryID uuid.UUID,
	rootSlug valueobjects.RootSlug,
	competence valueobjects.Competence,
	amountCents int64,
	occurredAt time.Time,
	now time.Time,
) (Expense, error) {
	if amountCents <= 0 {
		return Expense{}, ErrExpenseInvalidAmount
	}
	return Expense{
		id:                    uuid.New(),
		userID:                userID,
		source:                source,
		externalTransactionID: externalTransactionID,
		subcategoryID:         subcategoryID,
		rootSlug:              rootSlug,
		competence:            competence,
		amountCents:           amountCents,
		occurredAt:            occurredAt,
		version:               1,
		createdAt:             now,
		updatedAt:             now,
	}, nil
}

func HydrateExpense(
	id uuid.UUID,
	userID uuid.UUID,
	source valueobjects.ProducerSource,
	externalTransactionID valueobjects.ExternalTransactionID,
	subcategoryID uuid.UUID,
	rootSlug valueobjects.RootSlug,
	competence valueobjects.Competence,
	amountCents int64,
	occurredAt time.Time,
	version int64,
	tombstoneVersion *int64,
	deletedAt *time.Time,
	createdAt time.Time,
	updatedAt time.Time,
) Expense {
	return Expense{
		id:                    id,
		userID:                userID,
		source:                source,
		externalTransactionID: externalTransactionID,
		subcategoryID:         subcategoryID,
		rootSlug:              rootSlug,
		competence:            competence,
		amountCents:           amountCents,
		occurredAt:            occurredAt,
		version:               version,
		tombstoneVersion:      tombstoneVersion,
		deletedAt:             deletedAt,
		createdAt:             createdAt,
		updatedAt:             updatedAt,
	}
}

func (e Expense) ID() uuid.UUID                       { return e.id }
func (e Expense) UserID() uuid.UUID                   { return e.userID }
func (e Expense) Source() valueobjects.ProducerSource { return e.source }
func (e Expense) ExternalTransactionID() valueobjects.ExternalTransactionID {
	return e.externalTransactionID
}
func (e Expense) SubcategoryID() uuid.UUID            { return e.subcategoryID }
func (e Expense) RootSlug() valueobjects.RootSlug     { return e.rootSlug }
func (e Expense) Competence() valueobjects.Competence { return e.competence }
func (e Expense) AmountCents() int64                  { return e.amountCents }
func (e Expense) OccurredAt() time.Time               { return e.occurredAt }
func (e Expense) Version() int64                      { return e.version }
func (e Expense) TombstoneVersion() *int64            { return e.tombstoneVersion }
func (e Expense) DeletedAt() *time.Time               { return e.deletedAt }
func (e Expense) CreatedAt() time.Time                { return e.createdAt }
func (e Expense) UpdatedAt() time.Time                { return e.updatedAt }

func (e Expense) IsDeleted() bool { return e.deletedAt != nil }

func (e Expense) Identity() ExpenseIdentity {
	return ExpenseIdentity{
		UserID:                e.userID,
		Source:                e.source,
		ExternalTransactionID: e.externalTransactionID,
	}
}

func (e *Expense) Edit(
	subcategoryID uuid.UUID,
	rootSlug valueobjects.RootSlug,
	competence valueobjects.Competence,
	amountCents int64,
	occurredAt time.Time,
	expectedVersion int64,
	now time.Time,
) error {
	if e.IsDeleted() {
		return ErrExpenseAlreadyDeleted
	}
	if e.version != expectedVersion {
		return ErrExpenseVersionMismatch
	}
	if amountCents <= 0 {
		return ErrExpenseInvalidAmount
	}
	e.subcategoryID = subcategoryID
	e.rootSlug = rootSlug
	e.competence = competence
	e.amountCents = amountCents
	e.occurredAt = occurredAt
	e.version = e.version + 1
	e.updatedAt = now
	return nil
}

func (e *Expense) SoftDelete(expectedVersion int64, now time.Time) (int64, error) {
	if e.IsDeleted() {
		return 0, ErrExpenseAlreadyDeleted
	}
	if e.version != expectedVersion {
		return 0, ErrExpenseVersionMismatch
	}
	nextVersion := e.version + 1
	e.version = nextVersion
	e.tombstoneVersion = &nextVersion
	e.deletedAt = &now
	e.updatedAt = now
	return nextVersion, nil
}
