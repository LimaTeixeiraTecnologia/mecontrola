package services

import (
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

const MaxDeliveredAlerts = 10

type AlertStateResolver struct{}

func NewAlertStateResolver() *AlertStateResolver {
	return &AlertStateResolver{}
}

func (r *AlertStateResolver) Resolve(expenseCompetence, cutoff valueobjects.Competence, deliveredCount int) entities.AlertState {
	if expenseCompetence.Before(cutoff) {
		return entities.AlertStateSuppressedRetroactive
	}
	if deliveredCount >= MaxDeliveredAlerts {
		return entities.AlertStateRateLimited
	}
	return entities.AlertStateDelivered
}
