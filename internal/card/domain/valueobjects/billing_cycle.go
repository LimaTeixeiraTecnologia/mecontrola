package valueobjects

import (
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
)

type BillingCycle struct {
	ClosingDay int
	DueDay     int
}

func NewBillingCycle(closingDay, dueDay int) (BillingCycle, error) {
	if closingDay < 1 || closingDay > 31 {
		return BillingCycle{}, domain.ErrInvalidClosingDay
	}
	if dueDay < 1 || dueDay > 31 {
		return BillingCycle{}, domain.ErrInvalidDueDay
	}
	return BillingCycle{ClosingDay: closingDay, DueDay: dueDay}, nil
}
