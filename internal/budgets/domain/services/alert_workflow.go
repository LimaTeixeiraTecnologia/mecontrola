package services

import (
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type AlertDecision struct {
	State        entities.AlertState
	LogKey       string
	ErrorContext string
}

func IsRetroactiveAlert(expenseCompetence, cutoff valueobjects.Competence) bool {
	return expenseCompetence.Before(cutoff)
}

func DecideAlertForInsert(isRetroactive bool, deliveredCount int) AlertDecision {
	if isRetroactive {
		return AlertDecision{
			State:        entities.AlertStateSuppressedRetroactive,
			LogKey:       "budgets.usecase.evaluate_alert.suppressed_retroactive",
			ErrorContext: "inserir alerta retroativo",
		}
	}
	if deliveredCount >= MaxDeliveredAlerts {
		return AlertDecision{
			State:        entities.AlertStateRateLimited,
			LogKey:       "budgets.usecase.evaluate_alert.rate_limited",
			ErrorContext: "inserir alerta rate_limited",
		}
	}
	return AlertDecision{
		State:        entities.AlertStateDelivered,
		LogKey:       "",
		ErrorContext: "inserir alerta",
	}
}
