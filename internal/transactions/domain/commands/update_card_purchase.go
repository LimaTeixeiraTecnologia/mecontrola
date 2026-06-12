package commands

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type RawUpdateCardPurchase struct {
	PurchaseID        string
	TotalAmountCents  int64
	InstallmentsTotal int
	Description       string
	CategoryID        string
	SubcategoryID     string
	PurchasedAt       time.Time
	Version           int64
}

type UpdateCardPurchase struct {
	PurchaseID    uuid.UUID
	UserID        valueobjects.UserID
	TotalAmount   valueobjects.Money
	Installments  valueobjects.InstallmentCount
	Description   valueobjects.Description
	CategoryID    valueobjects.CategoryID
	SubcategoryID option.Option[valueobjects.SubcategoryID]
	PurchasedAt   time.Time
	Version       int64
}

func NewUpdateCardPurchase(raw RawUpdateCardPurchase, userID uuid.UUID) (UpdateCardPurchase, error) {
	var errs []error

	purchaseID, err := uuid.Parse(raw.PurchaseID)
	if err != nil {
		errs = append(errs, fmt.Errorf("purchase_id: %w", err))
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
		return UpdateCardPurchase{}, fmt.Errorf("commands/update_card_purchase: %w", errors.Join(errs...))
	}

	return UpdateCardPurchase{
		PurchaseID:    purchaseID,
		UserID:        valueobjects.UserIDFromUUID(userID),
		TotalAmount:   amount,
		Installments:  inst,
		Description:   desc,
		CategoryID:    catID,
		SubcategoryID: subID,
		PurchasedAt:   raw.PurchasedAt,
		Version:       raw.Version,
	}, nil
}
