package commands

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type RawUpdateTransaction struct {
	TransactionID string
	Direction     string
	PaymentMethod string
	AmountCents   int64
	Description   string
	CategoryID    string
	SubcategoryID string
	OccurredAt    time.Time
	Version       int64
}

type UpdateTransaction struct {
	TransactionID uuid.UUID
	UserID        valueobjects.UserID
	Direction     valueobjects.Direction
	PaymentMethod valueobjects.PaymentMethod
	Amount        valueobjects.Money
	Description   valueobjects.Description
	CategoryID    valueobjects.CategoryID
	SubcategoryID option.Option[valueobjects.SubcategoryID]
	OccurredAt    time.Time
	Version       int64
}

func NewUpdateTransaction(raw RawUpdateTransaction, userID uuid.UUID) (UpdateTransaction, error) {
	var errs []error

	txID, err := uuid.Parse(raw.TransactionID)
	if err != nil {
		errs = append(errs, fmt.Errorf("transaction_id: %w", err))
	}

	direction, err := valueobjects.ParseDirection(raw.Direction)
	if err != nil {
		errs = append(errs, fmt.Errorf("direction: %w", err))
	}

	pm, err := valueobjects.ParsePaymentMethodForCreate(raw.PaymentMethod)
	if err != nil {
		errs = append(errs, fmt.Errorf("payment_method: %w", err))
	}

	amount, err := valueobjects.NewMoney(raw.AmountCents)
	if err != nil {
		errs = append(errs, fmt.Errorf("amount_cents: %w", err))
	}

	desc, err := valueobjects.NewDescription(raw.Description)
	if err != nil {
		errs = append(errs, fmt.Errorf("description: %w", err))
	}

	catID, err := valueobjects.ParseCategoryID(raw.CategoryID)
	if err != nil {
		errs = append(errs, fmt.Errorf("category_id: %w", err))
	}

	if raw.OccurredAt.IsZero() {
		errs = append(errs, ErrCommandMissingOccurredAt)
	}

	var subID option.Option[valueobjects.SubcategoryID]
	if raw.SubcategoryID != "" {
		sid, sidErr := valueobjects.ParseSubcategoryID(raw.SubcategoryID)
		if sidErr != nil {
			errs = append(errs, fmt.Errorf("subcategory_id: %w", sidErr))
		} else {
			subID = option.Some(sid)
		}
	}

	if len(errs) > 0 {
		return UpdateTransaction{}, fmt.Errorf("commands/update_transaction: %w", errors.Join(errs...))
	}

	return UpdateTransaction{
		TransactionID: txID,
		UserID:        valueobjects.UserIDFromUUID(userID),
		Direction:     direction,
		PaymentMethod: pm,
		Amount:        amount,
		Description:   desc,
		CategoryID:    catID,
		SubcategoryID: subID,
		OccurredAt:    raw.OccurredAt,
		Version:       raw.Version,
	}, nil
}
