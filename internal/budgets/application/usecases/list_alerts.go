package usecases

import (
	"context"
	"errors"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
)

var ErrListAlertsInvalidUserID = errors.New("budgets: user_id inválido para listagem de alertas")

const defaultAlertLimit = 50
const maxAlertLimit = 200

type ListAlerts struct {
	alerts interfaces.AlertRepository
	uow    uow.UnitOfWork[output.ListAlertsOutput]
	o11y   observability.Observability
}

func NewListAlerts(
	alerts interfaces.AlertRepository,
	u uow.UnitOfWork[output.ListAlertsOutput],
	o11y observability.Observability,
) *ListAlerts {
	return &ListAlerts{
		alerts: alerts,
		uow:    u,
		o11y:   o11y,
	}
}

func (uc *ListAlerts) Execute(ctx context.Context, in input.ListAlertsInput) (output.ListAlertsOutput, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "budgets.usecase.list_alerts")
	defer span.End()

	userID, err := uuid.Parse(in.UserID)
	if err != nil {
		return output.ListAlertsOutput{}, ErrListAlertsInvalidUserID
	}

	limit := in.Limit
	if limit <= 0 {
		limit = defaultAlertLimit
	}
	if limit > maxAlertLimit {
		limit = maxAlertLimit
	}

	query := input.AlertQuery{
		Competence: in.Competence,
		RootSlug:   in.RootSlug,
		Threshold:  in.Threshold,
		Cursor:     in.Cursor,
		Limit:      limit,
	}

	result, execErr := uc.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (output.ListAlertsOutput, error) {
		items, nextCursor, listErr := uc.alerts.ListForUser(ctx, tx, userID, query)
		if listErr != nil {
			return output.ListAlertsOutput{}, fmt.Errorf("budgets.usecase.list_alerts: listar alertas: %w", listErr)
		}
		return mapListAlertsOutput(items, nextCursor), nil
	})

	if execErr != nil {
		span.RecordError(execErr)
		uc.o11y.Logger().Warn(ctx, "budgets.usecase.list_alerts.failed",
			observability.String("user_id", in.UserID),
			observability.Error(execErr),
		)
		return output.ListAlertsOutput{}, execErr
	}

	return result, nil
}

func mapListAlertsOutput(alerts []entities.Alert, nextCursor string) output.ListAlertsOutput {
	items := make([]output.AlertOutput, 0, len(alerts))
	for _, a := range alerts {
		items = append(items, mapAlertOutput(a))
	}
	return output.ListAlertsOutput{
		Alerts:     items,
		NextCursor: nextCursor,
	}
}

func mapAlertOutput(a entities.Alert) output.AlertOutput {
	return output.AlertOutput{
		ID:                     a.ID().String(),
		UserID:                 a.UserID().String(),
		Competence:             a.Competence().String(),
		RootSlug:               a.RootSlug().String(),
		Threshold:              a.Threshold().Int(),
		State:                  alertStateString(a.State()),
		TriggeredByCommittedAt: a.TriggeredByCommittedAt(),
		SpentCents:             a.SpentCents(),
		PlannedCents:           a.PlannedCents(),
		CreatedAt:              a.CreatedAt(),
	}
}

func alertStateString(s entities.AlertState) string {
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
