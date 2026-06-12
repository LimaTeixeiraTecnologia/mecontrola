package commands

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type RawCreateCardPurchase struct {
	CardID            string
	TotalAmountCents  int64
	InstallmentsTotal int
	Description       string
	CategoryID        string
	SubcategoryID     string
	PurchasedAt       time.Time
}

type CreateCardPurchase struct {
	UserID        valueobjects.UserID
	CardID        valueobjects.CardID
	TotalAmount   valueobjects.Money
	Installments  valueobjects.InstallmentCount
	Description   valueobjects.Description
	CategoryID    valueobjects.CategoryID
	SubcategoryID option.Option[valueobjects.SubcategoryID]
	PurchasedAt   time.Time
}

func NewCreateCardPurchase(raw RawCreateCardPurchase, userID uuid.UUID) (CreateCardPurchase, error) {
	var errs []error

	cardID, err := valueobjects.ParseCardID(raw.CardID)
	if err != nil {
		errs = append(errs, fmt.Errorf("card_id: %w", err))
	}

	amount, err := valueobjects.NewMoney(raw.TotalAmountCents)
	if err != nil {
		errs = append(errs, fmt.Errorf("total_amount_cents: %w", err))
	}

	inst, err := valueobjects.NewInstallmentCount(raw.InstallmentsTotal)
	if err != nil {
		errs = append(errs, fmt.Errorf("installments_total: %w", err))
	}

	desc, err := valueobjects.NewDescription(raw.Description)
	if err != nil {
		errs = append(errs, fmt.Errorf("description: %w", err))
	}

	catID, err := valueobjects.ParseCategoryID(raw.CategoryID)
	if err != nil {
		errs = append(errs, fmt.Errorf("category_id: %w", err))
	}

	if raw.PurchasedAt.IsZero() {
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
		return CreateCardPurchase{}, fmt.Errorf("commands/create_card_purchase: %w", errors.Join(errs...))
	}

	return CreateCardPurchase{
		UserID:        valueobjects.UserIDFromUUID(userID),
		CardID:        cardID,
		TotalAmount:   amount,
		Installments:  inst,
		Description:   desc,
		CategoryID:    catID,
		SubcategoryID: subID,
		PurchasedAt:   raw.PurchasedAt,
	}, nil
}
