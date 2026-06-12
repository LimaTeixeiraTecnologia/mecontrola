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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type GetMonthlySummary struct {
	factory interfaces.RepositoryFactory
	uow     uow.UnitOfWork[output.MonthlySummary]
	o11y    observability.Observability
}

func NewGetMonthlySummary(
	factory interfaces.RepositoryFactory,
	u uow.UnitOfWork[output.MonthlySummary],
	o11y observability.Observability,
) *GetMonthlySummary {
	return &GetMonthlySummary{
		factory: factory,
		uow:     u,
		o11y:    o11y,
	}
}

func (uc *GetMonthlySummary) Execute(ctx context.Context, refMonthStr string) (output.MonthlySummary, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "transactions.usecase.get_monthly_summary")
	defer span.End()

	principal, ok := auth.FromContext(ctx)
	if !ok {
		return output.MonthlySummary{}, ErrUsecaseUnauthorized
	}

	refMonth, err := valueobjects.NewRefMonth(refMonthStr)
	if err != nil {
		return output.MonthlySummary{}, fmt.Errorf("transactions/get_monthly_summary: ref_month inválido: %w", err)
	}

	result, execErr := uc.uow.Do(ctx, func(ctx context.Context, db database.DBTX) (output.MonthlySummary, error) {
		repo := uc.factory.MonthlySummaryRepository(db)
		summary, getErr := repo.Get(ctx, principal.UserID, refMonth)
		if getErr != nil {
			return output.MonthlySummary{}, fmt.Errorf("transactions/get_monthly_summary: buscar resumo: %w", getErr)
		}
		if summary == nil {
			zero := entities.NewMonthlySummary(principal.UserID, refMonth, 0, 0, 0, nil)
			return output.MonthlySummaryFrom(&zero), nil
		}
		return output.MonthlySummaryFrom(summary), nil
	})
	if execErr != nil {
		span.RecordError(execErr)
		return output.MonthlySummary{}, execErr
	}

	return result, nil
}
