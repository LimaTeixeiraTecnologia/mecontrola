package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/commands"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
)

type CreateRecurringTemplate struct {
	factory           interfaces.RepositoryFactory
	uow               uow.UnitOfWork
	categoryValidator interfaces.CategoryValidator
	publisher         interfaces.RecurringTemplateEventPublisher
	o11y              observability.Observability
}

func NewCreateRecurringTemplate(
	factory interfaces.RepositoryFactory,
	u uow.UnitOfWork,
	categoryValidator interfaces.CategoryValidator,
	publisher interfaces.RecurringTemplateEventPublisher,
	o11y observability.Observability,
) *CreateRecurringTemplate {
	return &CreateRecurringTemplate{
		factory:           factory,
		uow:               u,
		categoryValidator: categoryValidator,
		publisher:         publisher,
		o11y:              o11y,
	}
}

func (uc *CreateRecurringTemplate) Execute(ctx context.Context, raw input.RawCreateRecurringTemplate) (output.RecurringTemplate, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "transactions.usecase.create_recurring_template")
	defer span.End()

	principal, ok := auth.FromContext(ctx)
	if !ok {
		return output.RecurringTemplate{}, ErrUsecaseUnauthorized
	}

	startedAt, parseErr := parseISO8601(raw.StartedAt)
	if parseErr != nil {
		return output.RecurringTemplate{}, fmt.Errorf("transactions/create_recurring_template: started_at inválido: %w", parseErr)
	}

	rawCmd := commands.RawCreateRecurringTemplate{
		Direction:         raw.Direction,
		PaymentMethod:     raw.PaymentMethod,
		AmountCents:       raw.AmountCents,
		Description:       raw.Description,
		CategoryID:        raw.CategoryID.String(),
		Frequency:         raw.Frequency,
		DayOfMonth:        raw.DayOfMonth,
		StartedAt:         startedAt,
		InstallmentsTotal: raw.InstallmentsTotal,
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
			return output.RecurringTemplate{}, fmt.Errorf("transactions/create_recurring_template: ended_at inválido: %w", endErr)
		}
		rawCmd.EndedAt = &endedAt
	}

	cmd, cmdErr := commands.NewCreateRecurringTemplate(rawCmd, principal.UserID)
	if cmdErr != nil {
		span.RecordError(cmdErr)
		return output.RecurringTemplate{}, fmt.Errorf("transactions/create_recurring_template: comando: %w", cmdErr)
	}

	catSubID := optSubcategoryUUID(cmd.SubcategoryID)
	catSnap, catErr := uc.categoryValidator.Validate(ctx, cmd.CategoryID.UUID(), catSubID)
	if catErr != nil {
		span.RecordError(catErr)
		return output.RecurringTemplate{}, fmt.Errorf("transactions/create_recurring_template: validar categoria: %w", catErr)
	}

	templateID := uuid.New()
	eventID := uuid.New()
	now := time.Now().UTC()

	template := entities.NewRecurringTemplate(
		templateID,
		cmd.UserID,
		cmd.Direction,
		cmd.PaymentMethod,
		cmd.CardID,
		cmd.Amount,
		cmd.Description,
		cmd.CategoryID,
		cmd.SubcategoryID,
		catSnap.Name, snapSubName(catSubID, catSnap),
		cmd.Frequency,
		cmd.DayOfMonth,
		cmd.InstallmentsTotal,
		cmd.StartedAt,
		cmd.EndedAt,
		now,
	)

	t, execErr := uow.Do(ctx, uc.uow, func(ctx context.Context, db database.DBTX) (entities.RecurringTemplate, error) {
		repo := uc.factory.RecurringTemplateRepository(db)
		if createErr := repo.Create(ctx, &template); createErr != nil {
			return entities.RecurringTemplate{}, fmt.Errorf("transactions/create_recurring_template: persistir: %w", createErr)
		}
		evt := entities.RecurringTemplateCreated{
			EventID:     eventID,
			AggregateID: templateID,
			UserID:      cmd.UserID.UUID(),
			OccurredAt:  now,
		}
		if publishErr := uc.publisher.PublishCreated(ctx, db, evt); publishErr != nil {
			return entities.RecurringTemplate{}, fmt.Errorf("transactions/create_recurring_template: publicar evento: %w", publishErr)
		}
		return template, nil
	})
	if execErr != nil {
		span.RecordError(execErr)
		return output.RecurringTemplate{}, execErr
	}

	return output.RecurringTemplateFrom(&t), nil
}
