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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
)

type GetDaySummary struct {
	factory interfaces.RepositoryFactory
	uow     uow.UnitOfWork
	o11y    observability.Observability
}

func NewGetDaySummary(
	factory interfaces.RepositoryFactory,
	u uow.UnitOfWork,
	o11y observability.Observability,
) *GetDaySummary {
	return &GetDaySummary{
		factory: factory,
		uow:     u,
		o11y:    o11y,
	}
}

func (uc *GetDaySummary) Execute(ctx context.Context, day string) (output.DaySummary, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "transactions.usecase.get_day_summary")
	defer span.End()

	principal, ok := auth.FromContext(ctx)
	if !ok {
		return output.DaySummary{}, ErrUsecaseUnauthorized
	}

	if _, err := time.Parse("2006-01-02", day); err != nil {
		return output.DaySummary{}, fmt.Errorf("transactions/get_day_summary: dia inválido: %w", err)
	}

	result, execErr := uow.Do(ctx, uc.uow, func(ctx context.Context, db database.DBTX) (output.DaySummary, error) {
		repo := uc.factory.DayEntriesRepository(db)
		entries, listErr := repo.ListDayEntries(ctx, principal.UserID, day)
		if listErr != nil {
			return output.DaySummary{}, fmt.Errorf("transactions/get_day_summary: listar entradas do dia: %w", listErr)
		}

		summary := output.DaySummary{Day: day, Entries: make([]output.MonthlyEntry, 0, len(entries))}
		for _, e := range entries {
			var subID *string
			if e.SubcategoryID != nil {
				s := e.SubcategoryID.String()
				subID = &s
			}
			var catID string
			if e.CategoryID != uuid.Nil {
				catID = e.CategoryID.String()
			}
			summary.Entries = append(summary.Entries, output.MonthlyEntry{
				Kind:                    e.Kind,
				ID:                      e.ID,
				UserID:                  e.UserID.String(),
				RefMonth:                e.RefMonth,
				AmountCents:             e.AmountCents,
				Direction:               e.Direction,
				Description:             e.Description,
				CategoryID:              catID,
				SubcategoryID:           subID,
				CategoryNameSnapshot:    e.CategoryNameSnapshot,
				SubcategoryNameSnapshot: e.SubcategoryNameSnapshot,
				CreatedAt:               e.CreatedAt,
			})
			if e.Direction == "income" {
				summary.IncomeCents += e.AmountCents
				continue
			}
			summary.OutcomeCents += e.AmountCents
		}
		summary.TotalCents = summary.IncomeCents - summary.OutcomeCents
		return summary, nil
	})
	if execErr != nil {
		span.RecordError(execErr)
		return output.DaySummary{}, execErr
	}

	return result, nil
}
