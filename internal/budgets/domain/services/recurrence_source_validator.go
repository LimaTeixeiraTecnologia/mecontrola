package services

import (
	"errors"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
)

var ErrRecurrenceSourceNegativeTotal = errors.New("budgets.recurrence: competência sem valor total positivo não é fonte válida")

var ErrRecurrenceSourceAutoDraftWithoutAllocs = errors.New("budgets.recurrence: rascunho automático sem alocações não é fonte válida")

var ErrRecurrenceSourceDraftWithoutFullAllocs = errors.New("budgets.recurrence: rascunho manual com soma diferente de 100% não é fonte válida")

type RecurrenceSourceValidator struct{}

func NewRecurrenceSourceValidator() *RecurrenceSourceValidator {
	return &RecurrenceSourceValidator{}
}

func (v *RecurrenceSourceValidator) Validate(source entities.Budget) error {
	if source.TotalCents() <= 0 {
		return ErrRecurrenceSourceNegativeTotal
	}

	if source.AutoDraft() && len(source.Allocations()) == 0 {
		return ErrRecurrenceSourceAutoDraftWithoutAllocs
	}

	if source.IsDraft() {
		sum := 0
		for _, a := range source.Allocations() {
			sum += a.BasisPoints()
		}
		if sum != 10000 {
			return ErrRecurrenceSourceDraftWithoutFullAllocs
		}
	}

	return nil
}
