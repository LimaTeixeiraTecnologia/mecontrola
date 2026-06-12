package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type RecomputeMonthlySummaryInput struct {
	UserID   uuid.UUID
	RefMonth valueobjects.RefMonth
}

type RecomputeMonthlySummary struct {
	factory interfaces.RepositoryFactory
	uow     uow.UnitOfWork[struct{}]
	o11y    observability.Observability
}

func NewRecomputeMonthlySummary(
	factory interfaces.RepositoryFactory,
	u uow.UnitOfWork[struct{}],
	o11y observability.Observability,
) *RecomputeMonthlySummary {
	return &RecomputeMonthlySummary{
		factory: factory,
		uow:     u,
		o11y:    o11y,
	}
}

func (uc *RecomputeMonthlySummary) Execute(ctx context.Context, in RecomputeMonthlySummaryInput) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "transactions.usecase.recompute_monthly_summary")
	defer span.End()

	_, err := uc.uow.Do(ctx, func(ctx context.Context, db database.DBTX) (struct{}, error) {
		txRepo := uc.factory.TransactionRepository(db)
		invRepo := uc.factory.CardInvoiceRepository(db)
		summRepo := uc.factory.MonthlySummaryRepository(db)

		txIncome, txOutcome, err := txRepo.SumByMonth(ctx, in.UserID, in.RefMonth)
		if err != nil {
			return struct{}{}, fmt.Errorf("transactions/recompute_monthly_summary: somar lançamentos: %w", err)
		}

		invOutcome, err := invRepo.SumByMonth(ctx, in.UserID, in.RefMonth)
		if err != nil {
			return struct{}{}, fmt.Errorf("transactions/recompute_monthly_summary: somar faturas: %w", err)
		}

		totalOutcome := txOutcome + invOutcome
		if err := summRepo.Upsert(ctx, in.UserID, in.RefMonth, txIncome, totalOutcome, time.Now().UTC()); err != nil {
			return struct{}{}, fmt.Errorf("transactions/recompute_monthly_summary: upsert resumo: %w", err)
		}

		return struct{}{}, nil
	})
	if err != nil {
		span.RecordError(err)
		return err
	}
	return nil
}
