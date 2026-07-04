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
	CardID        string
	Installments  int
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
	CardID        option.Option[valueobjects.CardID]
	Installments  option.Option[valueobjects.InstallmentCount]
	OccurredAt    time.Time
}

func NewCreateTransaction(raw RawCreateTransaction, userID uuid.UUID) (CreateTransaction, error) {
	var errs []error

	direction, dirErr := valueobjects.ParseDirection(raw.Direction)
	if dirErr != nil {
		errs = append(errs, fmt.Errorf("direction: %w", dirErr))
	}

	pm, pmErr := valueobjects.ParsePaymentMethodForCreate(raw.PaymentMethod)
	if pmErr != nil {
		errs = append(errs, fmt.Errorf("payment_method: %w", pmErr))
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

	subID := parseOptionalSubcategoryID(raw.SubcategoryID, &errs)
	cardID := parseOptionalCardID(raw.CardID, &errs)
	installments := parseOptionalInstallmentCount(raw.Installments, &errs)
	validateCreditCardConstraints(pm, pmErr, cardID, direction, dirErr, &errs)

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
		CardID:        cardID,
		Installments:  installments,
		OccurredAt:    raw.OccurredAt,
	}, nil
}

func parseOptionalSubcategoryID(raw string, errs *[]error) option.Option[valueobjects.SubcategoryID] {
	if raw == "" {
		return option.None[valueobjects.SubcategoryID]()
	}
	sid, sidErr := valueobjects.ParseSubcategoryID(raw)
	if sidErr != nil {
		*errs = append(*errs, fmt.Errorf("subcategory_id: %w", sidErr))
		return option.None[valueobjects.SubcategoryID]()
	}
	return option.Some(sid)
}

func parseOptionalCardID(raw string, errs *[]error) option.Option[valueobjects.CardID] {
	if raw == "" {
		return option.None[valueobjects.CardID]()
	}
	cid, cidErr := valueobjects.ParseCardID(raw)
	if cidErr != nil {
		*errs = append(*errs, fmt.Errorf("card_id: %w", cidErr))
		return option.None[valueobjects.CardID]()
	}
	return option.Some(cid)
}

func parseOptionalInstallmentCount(raw int, errs *[]error) option.Option[valueobjects.InstallmentCount] {
	if raw == 0 {
		return option.None[valueobjects.InstallmentCount]()
	}
	ic, icErr := valueobjects.NewInstallmentCount(raw)
	if icErr != nil {
		*errs = append(*errs, fmt.Errorf("installments: %w", icErr))
		return option.None[valueobjects.InstallmentCount]()
	}
	return option.Some(ic)
}

func validateCreditCardConstraints(
	pm valueobjects.PaymentMethod,
	pmErr error,
	cardID option.Option[valueobjects.CardID],
	direction valueobjects.Direction,
	dirErr error,
	errs *[]error,
) {
	if pmErr != nil || pm != valueobjects.PaymentMethodCreditCard {
		return
	}
	if !cardID.IsPresent() {
		*errs = append(*errs, fmt.Errorf("card_id: %w", ErrCommandCreditCardRequiresCardID))
	}
	if dirErr == nil && direction != valueobjects.DirectionOutcome {
		*errs = append(*errs, fmt.Errorf("direction: %w", ErrCommandCreditCardRequiresOutcome))
	}
}
