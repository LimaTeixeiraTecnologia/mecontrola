package usecases

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
)

type RecurringTemplatePage struct {
	Templates  []output.RecurringTemplate
	NextCursor string
}

type ListRecurringTemplates struct {
	factory interfaces.RepositoryFactory
	uow     uow.UnitOfWork
	o11y    observability.Observability
}

func NewListRecurringTemplates(
	factory interfaces.RepositoryFactory,
	u uow.UnitOfWork,
	o11y observability.Observability,
) *ListRecurringTemplates {
	return &ListRecurringTemplates{
		factory: factory,
		uow:     u,
		o11y:    o11y,
	}
}

func (uc *ListRecurringTemplates) Execute(ctx context.Context, activeOnly bool, cursor string, limit int) (RecurringTemplatePage, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "transactions.usecase.list_recurring_templates")
	defer span.End()

	principal, ok := auth.FromContext(ctx)
	if !ok {
		return RecurringTemplatePage{}, ErrUsecaseUnauthorized
	}

	if limit <= 0 {
		limit = defaultListLimit
	}
	if limit > maxListLimit {
		limit = maxListLimit
	}

	result, execErr := uow.Do(ctx, uc.uow, func(ctx context.Context, db database.DBTX) (RecurringTemplatePage, error) {
		repo := uc.factory.RecurringTemplateRepository(db)
		templates, nextCursor, listErr := repo.List(ctx, principal.UserID, activeOnly, interfaces.Cursor{Value: cursor}, limit)
		if listErr != nil {
			return RecurringTemplatePage{}, fmt.Errorf("transactions/list_recurring_templates: listar: %w", listErr)
		}

		items := make([]output.RecurringTemplate, 0, len(templates))
		for _, t := range templates {
			items = append(items, output.RecurringTemplateFrom(t))
		}
		return RecurringTemplatePage{
			Templates:  items,
			NextCursor: nextCursor.Value,
		}, nil
	})
	if execErr != nil {
		span.RecordError(execErr)
		return RecurringTemplatePage{}, execErr
	}

	return result, nil
}
