package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type DeleteRecurringTemplate struct {
	factory   interfaces.RepositoryFactory
	uow       uow.UnitOfWork[struct{}]
	publisher interfaces.RecurringTemplateEventPublisher
	o11y      observability.Observability
}

func NewDeleteRecurringTemplate(
	factory interfaces.RepositoryFactory,
	u uow.UnitOfWork[struct{}],
	publisher interfaces.RecurringTemplateEventPublisher,
	o11y observability.Observability,
) *DeleteRecurringTemplate {
	return &DeleteRecurringTemplate{
		factory:   factory,
		uow:       u,
		publisher: publisher,
		o11y:      o11y,
	}
}

func (uc *DeleteRecurringTemplate) Execute(ctx context.Context, templateID string, version int64) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "transactions.usecase.delete_recurring_template")
	defer span.End()

	principal, ok := auth.FromContext(ctx)
	if !ok {
		return ErrUsecaseUnauthorized
	}

	parsedID, err := uuid.Parse(templateID)
	if err != nil {
		return fmt.Errorf("transactions/delete_recurring_template: template_id inválido: %w", err)
	}

	userID := valueobjects.UserIDFromUUID(principal.UserID)
	now := time.Now().UTC()
	eventID := uuid.New()

	_, execErr := uc.uow.Do(ctx, func(ctx context.Context, db database.DBTX) (struct{}, error) {
		repo := uc.factory.RecurringTemplateRepository(db)

		if softDelErr := repo.SoftDelete(ctx, parsedID, userID.UUID(), version, now); softDelErr != nil {
			return struct{}{}, fmt.Errorf("transactions/delete_recurring_template: soft-delete: %w", softDelErr)
		}

		evt := entities.RecurringTemplateDeleted{
			EventID:     eventID,
			AggregateID: parsedID,
			UserID:      userID.UUID(),
			OccurredAt:  now,
		}
		if publishErr := uc.publisher.PublishDeleted(ctx, db, evt); publishErr != nil {
			return struct{}{}, fmt.Errorf("transactions/delete_recurring_template: publicar evento: %w", publishErr)
		}
		return struct{}{}, nil
	})
	if execErr != nil {
		span.RecordError(execErr)
		return execErr
	}
	return nil
}
