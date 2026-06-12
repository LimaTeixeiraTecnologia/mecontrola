package commands

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type RawCreateTransaction struct {
	Direction     string
	PaymentMethod string
	AmountCents   int64
	Description   string
	CategoryID    string
	SubcategoryID string
	OccurredAt    time.Time
}

type CreateTransaction struct {
	UserID        valueobjects.UserID
	Direction     valueobjects.Direction
	PaymentMethod valueobjects.PaymentMethod
	Amount        valueobjects.Money
	Description   valueobjects.Description
	CategoryID    valueobjects.CategoryID
	SubcategoryID option.Option[valueobjects.SubcategoryID]
	OccurredAt    time.Time
}

func NewCreateTransaction(raw RawCreateTransaction, userID uuid.UUID) (CreateTransaction, error) {
	var errs []error

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
		return CreateTransaction{}, fmt.Errorf("commands/create_transaction: %w", errors.Join(errs...))
	}

	return CreateTransaction{
		UserID:        valueobjects.UserIDFromUUID(userID),
		Direction:     direction,
		PaymentMethod: pm,
		Amount:        amount,
		Description:   desc,
		CategoryID:    catID,
		SubcategoryID: subID,
		OccurredAt:    raw.OccurredAt,
	}, nil
}
