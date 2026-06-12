package valueobjects

import (
	"errors"
	"fmt"
	"time"
)

var ErrCardBillingSnapshotInvalid = errors.New("transactions: card billing snapshot inválido: dias devem estar entre 1 e 31")

type CardBillingSnapshot struct {
	closingDay DayOfMonth
	dueDay     DayOfMonth
}

func NewCardBillingSnapshot(closing, due int) (CardBillingSnapshot, error) {
	var errs []error

	closingDay, err := NewDayOfMonthSnapshot(closing)
	if err != nil {
		errs = append(errs, fmt.Errorf("closing_day: %w", err))
	}

	dueDay, err := NewDayOfMonthSnapshot(due)
	if err != nil {
		errs = append(errs, fmt.Errorf("due_day: %w", err))
	}

	if len(errs) > 0 {
		return CardBillingSnapshot{}, fmt.Errorf("transactions: card_billing_snapshot: %w", errors.Join(errs...))
	}

	return CardBillingSnapshot{closingDay: closingDay, dueDay: dueDay}, nil
}

func (s CardBillingSnapshot) ClosingDay() DayOfMonth { return s.closingDay }
func (s CardBillingSnapshot) DueDay() DayOfMonth     { return s.dueDay }

func (s CardBillingSnapshot) RefMonthForPurchase(purchasedAt time.Time) RefMonth {
	return RefMonthFromTime(purchasedAt, time.UTC)
}
