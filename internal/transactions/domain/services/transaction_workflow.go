package services

import (
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/commands"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type TransactionDecision struct {
	Transaction entities.Transaction
	Event       any
}

type TransactionWorkflow struct{}

func (w TransactionWorkflow) DecideCreate(
	cmd commands.CreateTransaction,
	txID uuid.UUID,
	eventID uuid.UUID,
	now time.Time,
) TransactionDecision {
	refMonth := valueobjects.RefMonthFromTime(cmd.OccurredAt, time.UTC)

	subcategoryID := uuid.Nil
	if sub, ok := cmd.SubcategoryID.Get(); ok {
		subcategoryID = sub.UUID()
	}

	tx := entities.NewTransaction(
		txID,
		cmd.UserID,
		cmd.Direction,
		cmd.PaymentMethod,
		cmd.Amount,
		cmd.Description,
		cmd.CategoryID,
		cmd.SubcategoryID,
		"",
		"",
		refMonth,
		cmd.OccurredAt,
		now,
	)

	evt := entities.TransactionCreated{
		EventID:       eventID,
		AggregateID:   txID,
		UserID:        cmd.UserID.UUID(),
		OccurredAt:    now,
		Direction:     cmd.Direction,
		PaymentMethod: cmd.PaymentMethod,
		AmountCents:   cmd.Amount.Cents(),
		RefMonth:      refMonth,
		CategoryID:    cmd.CategoryID.UUID(),
		SubcategoryID: subcategoryID,
	}

	return TransactionDecision{Transaction: tx, Event: evt}
}

func (w TransactionWorkflow) DecideUpdate(
	current entities.Transaction,
	cmd commands.UpdateTransaction,
	eventID uuid.UUID,
	now time.Time,
) TransactionDecision {
	newRefMonth := valueobjects.RefMonthFromTime(cmd.OccurredAt, time.UTC)
	oldRefMonth := current.RefMonth()

	current.Update(
		cmd.Direction,
		cmd.PaymentMethod,
		cmd.Amount,
		cmd.Description,
		cmd.CategoryID,
		cmd.SubcategoryID,
		"",
		"",
		newRefMonth,
		cmd.OccurredAt,
		now,
	)

	refMonthsAffected := dedupeRefMonths([]valueobjects.RefMonth{oldRefMonth, newRefMonth})

	evt := entities.TransactionUpdated{
		EventID:           eventID,
		AggregateID:       current.ID(),
		UserID:            cmd.UserID.UUID(),
		OccurredAt:        now,
		Direction:         cmd.Direction,
		PaymentMethod:     cmd.PaymentMethod,
		AmountCents:       cmd.Amount.Cents(),
		RefMonth:          newRefMonth,
		RefMonthsAffected: refMonthsAffected,
	}

	return TransactionDecision{Transaction: current, Event: evt}
}

func dedupeRefMonths(months []valueobjects.RefMonth) []valueobjects.RefMonth {
	seen := make(map[string]struct{}, len(months))
	result := make([]valueobjects.RefMonth, 0, len(months))
	for _, m := range months {
		if _, ok := seen[m.String()]; !ok {
			seen[m.String()] = struct{}{}
			result = append(result, m)
		}
	}
	return result
}
