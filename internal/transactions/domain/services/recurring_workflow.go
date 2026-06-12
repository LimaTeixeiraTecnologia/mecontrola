package services

import (
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type MaterializeDecision struct {
	ShouldMaterialize bool
	AsTransaction     bool
	AsCardPurchase    bool
	OccurredAt        time.Time
	RefMonth          valueobjects.RefMonth
	TemplateID        uuid.UUID
}

type RecurringWorkflow struct{}

func (w RecurringWorkflow) DecideMaterializeForDay(
	template entities.RecurringTemplate,
	today time.Time,
	loc *time.Location,
) MaterializeDecision {
	if template.DeletedAt() != nil {
		return MaterializeDecision{ShouldMaterialize: false}
	}

	if endedAt, ok := template.EndedAt().Get(); ok {
		if today.After(endedAt) {
			return MaterializeDecision{ShouldMaterialize: false}
		}
	}

	if today.Before(template.StartedAt()) {
		return MaterializeDecision{ShouldMaterialize: false}
	}

	dayInLoc := today.In(loc).Day()
	if dayInLoc != template.DayOfMonth().Value() {
		return MaterializeDecision{ShouldMaterialize: false}
	}

	occurredAt := time.Date(today.In(loc).Year(), today.In(loc).Month(), today.In(loc).Day(), 0, 0, 0, 0, loc)
	refMonth := valueobjects.RefMonthFromTime(occurredAt, loc)

	asTransaction := template.PaymentMethod() != valueobjects.PaymentMethodCreditCard
	asCardPurchase := template.PaymentMethod() == valueobjects.PaymentMethodCreditCard

	return MaterializeDecision{
		ShouldMaterialize: true,
		AsTransaction:     asTransaction,
		AsCardPurchase:    asCardPurchase,
		OccurredAt:        occurredAt,
		RefMonth:          refMonth,
		TemplateID:        template.ID(),
	}
}
