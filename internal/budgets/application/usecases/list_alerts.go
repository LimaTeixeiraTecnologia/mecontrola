package usecases

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/mappers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/commands"
)

type ListAlerts struct {
	factory interfaces.RepositoryFactory
	uow     uow.UnitOfWork[output.ListAlertsOutput]
	o11y    observability.Observability
}

func NewListAlerts(
	factory interfaces.RepositoryFactory,
	u uow.UnitOfWork[output.ListAlertsOutput],
	o11y observability.Observability,
) *ListAlerts {
	return &ListAlerts{
		factory: factory,
		uow:     u,
		o11y:    o11y,
	}
}

func (uc *ListAlerts) Execute(ctx context.Context, in input.ListAlertsInput) (output.ListAlertsOutput, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "budgets.usecase.list_alerts")
	defer span.End()

	limit := in.Limit
	if limit < 0 {
		limit = 0
	}
	cmd, err := commands.NewListAlertsCommand(in.UserID, in.Cursor, limit)
	if err != nil {
		span.RecordError(err)
		return output.ListAlertsOutput{}, ErrListAlertsInvalidUserID
	}

	query := input.AlertQuery{
		Competence: in.Competence,
		RootSlug:   in.RootSlug,
		Threshold:  in.Threshold,
		Cursor:     cmd.Cursor,
		Limit:      cmd.Limit,
	}

	result, execErr := uc.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (output.ListAlertsOutput, error) {
		alerts := uc.factory.AlertRepository(tx)
		items, nextCursor, listErr := alerts.ListForUser(ctx, cmd.UserID, query)
		if listErr != nil {
			return output.ListAlertsOutput{}, fmt.Errorf("budgets.usecase.list_alerts: listar alertas: %w", listErr)
		}
		return mappers.M.ListAlerts(items, nextCursor), nil
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
