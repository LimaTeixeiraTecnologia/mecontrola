package commands

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type RawCreateRecurringTemplate struct {
	Direction         string
	PaymentMethod     string
	CardID            string
	AmountCents       int64
	Description       string
	CategoryID        string
	SubcategoryID     string
	Frequency         string
	DayOfMonth        int
	StartedAt         time.Time
	EndedAt           *time.Time
	InstallmentsTotal int
}

type CreateRecurringTemplate struct {
	UserID            valueobjects.UserID
	Direction         valueobjects.Direction
	PaymentMethod     valueobjects.PaymentMethod
	CardID            option.Option[valueobjects.CardID]
	Amount            valueobjects.Money
	Description       valueobjects.Description
	CategoryID        valueobjects.CategoryID
	SubcategoryID     option.Option[valueobjects.SubcategoryID]
	Frequency         valueobjects.Frequency
	DayOfMonth        valueobjects.DayOfMonth
	StartedAt         time.Time
	EndedAt           option.Option[time.Time]
	InstallmentsTotal valueobjects.InstallmentCount
}

func NewCreateRecurringTemplate(raw RawCreateRecurringTemplate, userID uuid.UUID) (CreateRecurringTemplate, error) { //nolint:revive // smart constructor valida 10+ campos; complexidade é estrutural, não acidental
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

	freq, err := valueobjects.ParseFrequency(raw.Frequency)
	if err != nil {
		errs = append(errs, fmt.Errorf("frequency: %w", err))
	}

	dom, err := valueobjects.NewDayOfMonth(raw.DayOfMonth)
	if err != nil {
		errs = append(errs, fmt.Errorf("day_of_month: %w", err))
	}

	installments := 1
	if raw.InstallmentsTotal > 0 {
		installments = raw.InstallmentsTotal
	}
	instCount, err := valueobjects.NewInstallmentCount(installments)
	if err != nil {
		errs = append(errs, fmt.Errorf("installments_total: %w", err))
	}

	var cardIDOpt option.Option[valueobjects.CardID]
	if raw.CardID != "" {
		cid, cidErr := valueobjects.ParseCardID(raw.CardID)
		if cidErr != nil {
			errs = append(errs, fmt.Errorf("card_id: %w", cidErr))
		} else {
			cardIDOpt = option.Some(cid)
		}
	}

	if pm == valueobjects.PaymentMethodCreditCard && !cardIDOpt.IsPresent() {
		errs = append(errs, ErrCommandCreditCardRequiresCardID)
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

	var endedAtOpt option.Option[time.Time]
	if raw.EndedAt != nil {
		endedAtOpt = option.Some(*raw.EndedAt)
	}

	if len(errs) > 0 {
		return CreateRecurringTemplate{}, fmt.Errorf("commands/create_recurring_template: %w", errors.Join(errs...))
	}

	return CreateRecurringTemplate{
		UserID:            valueobjects.UserIDFromUUID(userID),
		Direction:         direction,
		PaymentMethod:     pm,
		CardID:            cardIDOpt,
		Amount:            amount,
		Description:       desc,
		CategoryID:        catID,
		SubcategoryID:     subID,
		Frequency:         freq,
		DayOfMonth:        dom,
		StartedAt:         raw.StartedAt,
		EndedAt:           endedAtOpt,
		InstallmentsTotal: instCount,
	}, nil
}
