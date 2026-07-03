package binding

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	agentsifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	txinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/input"
	txusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases"
)

type recurrenceManagerAdapter struct {
	createRT *txusecases.CreateRecurringTemplate
	updateRT *txusecases.UpdateRecurringTemplate
	deleteRT *txusecases.DeleteRecurringTemplate
	listRT   *txusecases.ListRecurringTemplates
	o11y     observability.Observability
}

func NewRecurrenceManagerAdapter(
	createRT *txusecases.CreateRecurringTemplate,
	updateRT *txusecases.UpdateRecurringTemplate,
	deleteRT *txusecases.DeleteRecurringTemplate,
	listRT *txusecases.ListRecurringTemplates,
	o11y observability.Observability,
) agentsifaces.RecurrenceManager {
	return &recurrenceManagerAdapter{
		createRT: createRT,
		updateRT: updateRT,
		deleteRT: deleteRT,
		listRT:   listRT,
		o11y:     o11y,
	}
}

func (a *recurrenceManagerAdapter) CreateRecurrence(ctx context.Context, in agentsifaces.RawRecurrence) (agentsifaces.EntryRef, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.recurrence_manager.create_recurrence")
	defer span.End()

	out, err := a.createRT.Execute(ctx, txinput.RawCreateRecurringTemplate{
		Direction:     in.Direction,
		PaymentMethod: in.PaymentMethod,
		CardID:        in.CardID,
		AmountCents:   in.AmountCents,
		Description:   in.Description,
		CategoryID:    in.CategoryID,
		SubcategoryID: in.SubcategoryID,
		Frequency:     in.Frequency,
		DayOfMonth:    in.DayOfMonth,
		StartedAt:     in.StartedAt,
	})
	if err != nil {
		span.RecordError(err)
		return agentsifaces.EntryRef{}, fmt.Errorf("agents/binding/recurrence_manager: criar recorrência: %w", err)
	}
	return agentsifaces.EntryRef{ID: out.ID, Kind: "recurring_template"}, nil
}

func (a *recurrenceManagerAdapter) UpdateRecurrence(ctx context.Context, templateID string, in agentsifaces.RawUpdateRecurrence) (agentsifaces.EntryRef, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.recurrence_manager.update_recurrence")
	defer span.End()

	raw := txinput.RawUpdateRecurringTemplate{
		Version: in.Version,
	}
	if in.Direction != nil {
		raw.Direction = *in.Direction
	}
	if in.PaymentMethod != nil {
		raw.PaymentMethod = *in.PaymentMethod
	}
	if in.AmountCents != nil {
		raw.AmountCents = *in.AmountCents
	}
	if in.Description != nil {
		raw.Description = *in.Description
	}
	if in.CategoryID != nil {
		raw.CategoryID = *in.CategoryID
	}
	raw.SubcategoryID = in.SubcategoryID
	if in.Frequency != nil {
		raw.Frequency = *in.Frequency
	}
	if in.DayOfMonth != nil {
		raw.DayOfMonth = *in.DayOfMonth
	}
	raw.EndedAt = in.EndedAt

	out, err := a.updateRT.Execute(ctx, templateID, raw)
	if err != nil {
		span.RecordError(err)
		return agentsifaces.EntryRef{}, fmt.Errorf("agents/binding/recurrence_manager: atualizar recorrência: %w", err)
	}
	return agentsifaces.EntryRef{ID: out.ID, Kind: "recurring_template"}, nil
}

func (a *recurrenceManagerAdapter) DeleteRecurrence(ctx context.Context, templateID string, version int64) error {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.recurrence_manager.delete_recurrence")
	defer span.End()

	if err := a.deleteRT.Execute(ctx, templateID, version); err != nil {
		span.RecordError(err)
		return fmt.Errorf("agents/binding/recurrence_manager: deletar recorrência: %w", err)
	}
	return nil
}

func (a *recurrenceManagerAdapter) ListRecurrences(ctx context.Context, activeOnly bool, cursor string, limit int) ([]agentsifaces.Recurrence, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.recurrence_manager.list_recurrences")
	defer span.End()

	page, err := a.listRT.Execute(ctx, activeOnly, cursor, limit)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("agents/binding/recurrence_manager: listar recorrências: %w", err)
	}

	result := make([]agentsifaces.Recurrence, 0, len(page.Templates))
	for _, t := range page.Templates {
		var endedAt *time.Time
		if t.EndedAt != nil {
			endedAt = t.EndedAt
		}
		result = append(result, agentsifaces.Recurrence{
			ID:                      t.ID,
			UserID:                  t.UserID,
			Direction:               t.Direction,
			PaymentMethod:           t.PaymentMethod,
			CardID:                  t.CardID,
			AmountCents:             t.AmountCents,
			Description:             t.Description,
			CategoryID:              t.CategoryID,
			SubcategoryID:           t.SubcategoryID,
			CategoryNameSnapshot:    t.CategoryNameSnapshot,
			SubcategoryNameSnapshot: t.SubcategoryNameSnapshot,
			Frequency:               t.Frequency,
			DayOfMonth:              t.DayOfMonth,
			InstallmentsTotal:       t.InstallmentsTotal,
			StartedAt:               t.StartedAt,
			EndedAt:                 endedAt,
			Version:                 t.Version,
			CreatedAt:               t.CreatedAt,
			UpdatedAt:               t.UpdatedAt,
		})
	}
	return result, nil
}
