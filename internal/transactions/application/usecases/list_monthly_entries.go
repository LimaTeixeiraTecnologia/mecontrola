package usecases

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type ListMonthlyEntries struct {
	factory interfaces.RepositoryFactory
	uow     uow.UnitOfWork[output.MonthlyEntriesPage]
	o11y    observability.Observability
}

func NewListMonthlyEntries(
	factory interfaces.RepositoryFactory,
	u uow.UnitOfWork[output.MonthlyEntriesPage],
	o11y observability.Observability,
) *ListMonthlyEntries {
	return &ListMonthlyEntries{
		factory: factory,
		uow:     u,
		o11y:    o11y,
	}
}

func (uc *ListMonthlyEntries) Execute(ctx context.Context, refMonthStr, cursor string, limit int) (output.MonthlyEntriesPage, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "transactions.usecase.list_monthly_entries")
	defer span.End()

	principal, ok := auth.FromContext(ctx)
	if !ok {
		return output.MonthlyEntriesPage{}, ErrUsecaseUnauthorized
	}

	refMonth, err := valueobjects.NewRefMonth(refMonthStr)
	if err != nil {
		return output.MonthlyEntriesPage{}, fmt.Errorf("transactions/list_monthly_entries: ref_month inválido: %w", err)
	}

	if limit <= 0 {
		limit = defaultListLimit
	}
	if limit > maxListLimit {
		limit = maxListLimit
	}

	result, execErr := uc.uow.Do(ctx, func(ctx context.Context, db database.DBTX) (output.MonthlyEntriesPage, error) {
		repo := uc.factory.MonthlySummaryRepository(db)
		entries, nextCursor, listErr := repo.ListEntries(ctx, principal.UserID, refMonth, interfaces.Cursor{Value: cursor}, limit)
		if listErr != nil {
			return output.MonthlyEntriesPage{}, fmt.Errorf("transactions/list_monthly_entries: listar entradas: %w", listErr)
		}

		items := make([]any, 0, len(entries))
		for _, e := range entries {
			items = append(items, output.MonthlyEntry{
				Kind:        e.Kind,
				ID:          e.ID,
				UserID:      e.UserID.String(),
				RefMonth:    e.RefMonth,
				AmountCents: e.AmountCents,
				Direction:   e.Direction,
				Description: e.Description,
				CreatedAt:   e.CreatedAt,
			})
		}

		return output.MonthlyEntriesPage{
			Items:      items,
			NextCursor: nextCursor.Value,
			HasMore:    nextCursor.Value != "",
		}, nil
	})
	if execErr != nil {
		span.RecordError(execErr)
		return output.MonthlyEntriesPage{}, execErr
	}

	return result, nil
}
