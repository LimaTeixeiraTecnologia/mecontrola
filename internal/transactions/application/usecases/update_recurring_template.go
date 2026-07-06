package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/commands"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
)

type UpdateRecurringTemplate struct {
	factory           interfaces.RepositoryFactory
	uow               uow.UnitOfWork
	categoryValidator interfaces.CategoryValidator
	categoryGate      interfaces.CategoryWriteGate
	o11y              observability.Observability
}

func NewUpdateRecurringTemplate(
	factory interfaces.RepositoryFactory,
	u uow.UnitOfWork,
	categoryValidator interfaces.CategoryValidator,
	categoryGate interfaces.CategoryWriteGate,
	o11y observability.Observability,
) *UpdateRecurringTemplate {
	return &UpdateRecurringTemplate{
		factory:           factory,
		uow:               u,
		categoryValidator: categoryValidator,
		categoryGate:      categoryGate,
		o11y:              o11y,
	}
}

func (uc *UpdateRecurringTemplate) Execute(ctx context.Context, templateID string, raw input.RawUpdateRecurringTemplate) (output.RecurringTemplate, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "transactions.usecase.update_recurring_template")
	defer span.End()

	if err := raw.Validate(); err != nil {
		return output.RecurringTemplate{}, err
	}

	principal, ok := auth.FromContext(ctx)
	if !ok {
		return output.RecurringTemplate{}, ErrUsecaseUnauthorized
	}

	rawCmd, rawErr := buildRawUpdateRecurringTemplate(raw, templateID)
	if rawErr != nil {
		return output.RecurringTemplate{}, fmt.Errorf("transactions/update_recurring_template: %w", rawErr)
	}

	cmd, cmdErr := commands.NewUpdateRecurringTemplate(rawCmd, principal.UserID)
	if cmdErr != nil {
		span.RecordError(cmdErr)
		return output.RecurringTemplate{}, fmt.Errorf("transactions/update_recurring_template: comando: %w", cmdErr)
	}

	if guardErr := guardSubcategoryRequired(cmd.Direction, cmd.SubcategoryID.IsPresent()); guardErr != nil {
		return output.RecurringTemplate{}, guardErr
	}

	catSubID := optSubcategoryUUID(cmd.SubcategoryID)
	catSnap, catErr := uc.categoryValidator.Validate(ctx, cmd.CategoryID.UUID(), catSubID)
	if catErr != nil {
		span.RecordError(catErr)
		return output.RecurringTemplate{}, fmt.Errorf("transactions/update_recurring_template: validar categoria: %w", catErr)
	}

	evidence, gateErr := approveUpdateCategory(ctx, uc.categoryGate, categoryEvidence{}, cmd.Direction.String(), "update_recurring_template", cmd.CategoryID.UUID(), catSubID)
	if gateErr != nil {
		span.RecordError(gateErr)
		return output.RecurringTemplate{}, fmt.Errorf("transactions/update_recurring_template: gate de categoria: %w", gateErr)
	}

	now := time.Now().UTC()

	t, execErr := uow.Do(ctx, uc.uow, func(ctx context.Context, db database.DBTX) (entities.RecurringTemplate, error) {
		repo := uc.factory.RecurringTemplateRepository(db)

		current, getErr := repo.GetByID(ctx, cmd.TemplateID, cmd.UserID.UUID())
		if getErr != nil {
			return entities.RecurringTemplate{}, fmt.Errorf("transactions/update_recurring_template: buscar template: %w", getErr)
		}

		current.Update(
			cmd.Direction, cmd.PaymentMethod, cmd.CardID,
			cmd.Amount, cmd.Description, cmd.CategoryID, cmd.SubcategoryID,
			catSnap.Name, snapSubName(catSubID, catSnap),
			evidence,
			cmd.Frequency, cmd.DayOfMonth, cmd.InstallmentsTotal,
			cmd.StartedAt, cmd.EndedAt, now,
		)

		if updateErr := repo.UpdateWithVersion(ctx, current, cmd.Version); updateErr != nil {
			return entities.RecurringTemplate{}, fmt.Errorf("transactions/update_recurring_template: atualizar: %w", updateErr)
		}

		return *current, nil
	})
	if execErr != nil {
		span.RecordError(execErr)
		return output.RecurringTemplate{}, execErr
	}

	return output.RecurringTemplateFrom(&t), nil
}

func buildRawUpdateRecurringTemplate(raw input.RawUpdateRecurringTemplate, templateID string) (commands.RawUpdateRecurringTemplate, error) {
	startedAt, parseErr := parseISO8601(raw.StartedAt)
	if parseErr != nil {
		return commands.RawUpdateRecurringTemplate{}, fmt.Errorf("started_at inválido: %w", parseErr)
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
			return commands.RawUpdateRecurringTemplate{}, fmt.Errorf("ended_at inválido: %w", endErr)
		}
		rawCmd.EndedAt = &endedAt
	}
	return rawCmd, nil
}
