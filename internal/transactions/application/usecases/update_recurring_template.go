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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/commands"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
)

type UpdateRecurringTemplate struct {
	factory           interfaces.RepositoryFactory
	uow               uow.UnitOfWork[entities.RecurringTemplate]
	categoryValidator interfaces.CategoryValidator
	publisher         interfaces.RecurringTemplateEventPublisher
	o11y              observability.Observability
}

func NewUpdateRecurringTemplate(
	factory interfaces.RepositoryFactory,
	u uow.UnitOfWork[entities.RecurringTemplate],
	categoryValidator interfaces.CategoryValidator,
	publisher interfaces.RecurringTemplateEventPublisher,
	o11y observability.Observability,
) *UpdateRecurringTemplate {
	return &UpdateRecurringTemplate{
		factory:           factory,
		uow:               u,
		categoryValidator: categoryValidator,
		publisher:         publisher,
		o11y:              o11y,
	}
}

func (uc *UpdateRecurringTemplate) Execute(ctx context.Context, templateID string, raw input.RawUpdateRecurringTemplate) (output.RecurringTemplate, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "transactions.usecase.update_recurring_template")
	defer span.End()

	principal, ok := auth.FromContext(ctx)
	if !ok {
		return output.RecurringTemplate{}, ErrUsecaseUnauthorized
	}

	startedAt, parseErr := parseISO8601(raw.StartedAt)
	if parseErr != nil {
		return output.RecurringTemplate{}, fmt.Errorf("transactions/update_recurring_template: started_at inválido: %w", parseErr)
	}

	rawCmd := commands.RawUpdateRecurringTemplate{
		TemplateID:        templateID,
		Direction:         raw.Direction,
		PaymentMethod:     raw.PaymentMethod,
		AmountCents:       raw.AmountCents,
		Description:       raw.Description,
		CategoryID:        raw.CategoryID.String(),
		Frequency:         raw.Frequency,
		DayOfMonth:        raw.DayOfMonth,
		StartedAt:         startedAt,
		InstallmentsTotal: raw.InstallmentsTotal,
		Version:           raw.Version,
	}
	if raw.CardID != nil {
		rawCmd.CardID = raw.CardID.String()
	}
	if raw.SubcategoryID != nil {
		rawCmd.SubcategoryID = raw.SubcategoryID.String()
	}
	if raw.EndedAt != nil {
		endedAt, endErr := parseISO8601(*raw.EndedAt)
		if endErr != nil {
			return output.RecurringTemplate{}, fmt.Errorf("transactions/update_recurring_template: ended_at inválido: %w", endErr)
		}
		rawCmd.EndedAt = &endedAt
	}

	cmd, cmdErr := commands.NewUpdateRecurringTemplate(rawCmd, principal.UserID)
	if cmdErr != nil {
		span.RecordError(cmdErr)
		return output.RecurringTemplate{}, fmt.Errorf("transactions/update_recurring_template: comando: %w", cmdErr)
	}

	catSubID := optSubcategoryUUID(cmd.SubcategoryID)
	catSnap, catErr := uc.categoryValidator.Validate(ctx, cmd.CategoryID.UUID(), catSubID)
	if catErr != nil {
		span.RecordError(catErr)
		return output.RecurringTemplate{}, fmt.Errorf("transactions/update_recurring_template: validar categoria: %w", catErr)
	}

	eventID := uuid.New()
	now := time.Now().UTC()

	t, execErr := uc.uow.Do(ctx, func(ctx context.Context, db database.DBTX) (entities.RecurringTemplate, error) {
		repo := uc.factory.RecurringTemplateRepository(db)

		current, getErr := repo.GetByID(ctx, cmd.TemplateID, cmd.UserID.UUID())
		if getErr != nil {
			return entities.RecurringTemplate{}, fmt.Errorf("transactions/update_recurring_template: buscar template: %w", getErr)
		}

		current.Update(
			cmd.Direction, cmd.PaymentMethod, cmd.CardID,
			cmd.Amount, cmd.Description, cmd.CategoryID, cmd.SubcategoryID,
			catSnap.Name, snapSubName(catSubID, catSnap),
			cmd.Frequency, cmd.DayOfMonth, cmd.InstallmentsTotal,
			cmd.StartedAt, cmd.EndedAt, now,
		)

		if updateErr := repo.UpdateWithVersion(ctx, current, cmd.Version); updateErr != nil {
			return entities.RecurringTemplate{}, fmt.Errorf("transactions/update_recurring_template: atualizar: %w", updateErr)
		}

		evt := entities.RecurringTemplateUpdated{
			EventID:     eventID,
			AggregateID: cmd.TemplateID,
			UserID:      cmd.UserID.UUID(),
			OccurredAt:  now,
		}
		if publishErr := uc.publisher.PublishUpdated(ctx, db, evt); publishErr != nil {
			return entities.RecurringTemplate{}, fmt.Errorf("transactions/update_recurring_template: publicar evento: %w", publishErr)
		}
		return *current, nil
	})
	if execErr != nil {
		span.RecordError(execErr)
		return output.RecurringTemplate{}, execErr
	}

	return output.RecurringTemplateFrom(&t), nil
}
