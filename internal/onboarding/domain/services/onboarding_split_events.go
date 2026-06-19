package services

import (
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

func BuildSplitsCalculatedFromAllocation(
	userID uuid.UUID,
	channel string,
	incomeCents int64,
	allocation valueobjects.BudgetAllocation,
	eventID uuid.UUID,
	occurredAt time.Time,
) entities.SplitsCalculated {
	allocations := allocation.Allocations()
	entriesList := make([]entities.SplitsCalculatedEntry, 0, len(allocations))
	for _, a := range allocations {
		entriesList = append(entriesList, entities.SplitsCalculatedEntry{
			Kind:    a.Kind.String(),
			Percent: a.BasisPoints / 100,
		})
	}
	return entities.SplitsCalculated{
		EventID:     eventID,
		UserID:      userID,
		Channel:     channel,
		IncomeCents: incomeCents,
		Allocations: entriesList,
		OccurredAt:  occurredAt,
	}
}
