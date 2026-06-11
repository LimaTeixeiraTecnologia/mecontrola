package mappers

import (
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
)

func (m Mapper) Alert(a entities.Alert) output.AlertOutput {
	return output.AlertOutput{
		ID:                     a.ID().String(),
		UserID:                 a.UserID().String(),
		Competence:             a.Competence().String(),
		RootSlug:               a.RootSlug().String(),
		Threshold:              a.Threshold().Int(),
		State:                  m.AlertStateString(a.State()),
		TriggeredByCommittedAt: a.TriggeredByCommittedAt(),
		SpentCents:             a.SpentCents(),
		PlannedCents:           a.PlannedCents(),
		CreatedAt:              a.CreatedAt(),
	}
}

func (m Mapper) Alerts(as []entities.Alert) []output.AlertOutput {
	items := make([]output.AlertOutput, 0, len(as))
	for _, a := range as {
		items = append(items, m.Alert(a))
	}
	return items
}

func (m Mapper) ListAlerts(items []entities.Alert, nextCursor string) output.ListAlertsOutput {
	return output.ListAlertsOutput{
		Alerts:     m.Alerts(items),
		NextCursor: nextCursor,
	}
}

func (Mapper) AlertStateString(s entities.AlertState) string {
	switch s {
	case entities.AlertStatePendingDelivery:
		return "pending_delivery"
	case entities.AlertStateDelivered:
		return "delivered"
	case entities.AlertStateSuppressedStale:
		return "suppressed_stale"
	case entities.AlertStateSuppressedRetroactive:
		return "suppressed_retroactive"
	case entities.AlertStateRateLimited:
		return "rate_limited"
	default:
		return ""
	}
}
