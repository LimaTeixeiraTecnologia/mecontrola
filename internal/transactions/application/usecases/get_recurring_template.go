package usecases

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
)

type GetRecurringTemplate struct {
	factory interfaces.RepositoryFactory
	uow     uow.UnitOfWork
	o11y    observability.Observability
}

func NewGetRecurringTemplate(
	factory interfaces.RepositoryFactory,
	u uow.UnitOfWork,
	o11y observability.Observability,
) *GetRecurringTemplate {
	return &GetRecurringTemplate{
		factory: factory,
		uow:     u,
		o11y:    o11y,
	}
}

func (uc *GetRecurringTemplate) Execute(ctx context.Context, templateID string) (output.RecurringTemplate, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "transactions.usecase.get_recurring_template")
	defer span.End()

	principal, ok := auth.FromContext(ctx)
	if !ok {
		return output.RecurringTemplate{}, ErrUsecaseUnauthorized
	}

	parsedID, err := uuid.Parse(templateID)
	if err != nil {
		return output.RecurringTemplate{}, fmt.Errorf("transactions/get_recurring_template: template_id inválido: %w", err)
	}

	result, execErr := uow.Do(ctx, uc.uow, func(ctx context.Context, db database.DBTX) (output.RecurringTemplate, error) {
		repo := uc.factory.RecurringTemplateRepository(db)
		t, getErr := repo.GetByID(ctx, parsedID, principal.UserID)
		if getErr != nil {
			return output.RecurringTemplate{}, fmt.Errorf("transactions/get_recurring_template: buscar template: %w", getErr)
		}
		return output.RecurringTemplateFrom(t), nil
	})
	if execErr != nil {
		span.RecordError(execErr)
		return output.RecurringTemplate{}, execErr
	}

	return result, nil
}
