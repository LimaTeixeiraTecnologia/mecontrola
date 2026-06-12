package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type ReconcileMonthlySummary struct {
	db            database.DBTX
	factory       interfaces.RepositoryFactory
	lookbackHours int
	o11y          observability.Observability
}

func NewReconcileMonthlySummary(
	db database.DBTX,
	factory interfaces.RepositoryFactory,
	lookbackHours int,
	o11y observability.Observability,
) *ReconcileMonthlySummary {
	return &ReconcileMonthlySummary{
		db:            db,
		factory:       factory,
		lookbackHours: lookbackHours,
		o11y:          o11y,
	}
}

func (uc *ReconcileMonthlySummary) Execute(ctx context.Context) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "transactions.usecase.reconcile_monthly_summary")
	defer span.End()

	since := time.Now().UTC().Add(-time.Duration(uc.lookbackHours) * time.Hour)
	cursor := interfaces.Cursor{}

	for {
		summaryRepo := uc.factory.MonthlySummaryRepository(uc.db)

		keys, nextCursor, err := summaryRepo.ListActiveSince(ctx, since, cursor, materializeBatchSize)
		if err != nil {
			span.RecordError(err)
			return fmt.Errorf("transactions/reconcile_monthly_summary: listar ativos: %w", err)
		}

		for _, key := range keys {
			if reconcileErr := uc.reconcileKey(ctx, key); reconcileErr != nil {
				span.RecordError(reconcileErr)
				return fmt.Errorf("transactions/reconcile_monthly_summary: reconciliar (%s, %s): %w", key.UserID, key.RefMonth, reconcileErr)
			}
		}

		if nextCursor.Value == "" {
			break
		}
		cursor = nextCursor
	}

	return nil
}

func (uc *ReconcileMonthlySummary) reconcileKey(ctx context.Context, key interfaces.MonthlySummaryKey) error {
	refMonth, err := valueobjects.NewRefMonth(key.RefMonth)
	if err != nil {
		return fmt.Errorf("transactions/reconcile: ref_month inválido %s: %w", key.RefMonth, err)
	}

	userID := key.UserID

	txRepo := uc.factory.TransactionRepository(uc.db)
	invoiceRepo := uc.factory.CardInvoiceRepository(uc.db)
	summaryRepo := uc.factory.MonthlySummaryRepository(uc.db)

	income, outcome, err := txRepo.SumByMonth(ctx, userID, refMonth)
	if err != nil {
		return fmt.Errorf("transactions/reconcile: soma transações: %w", err)
	}

	cardOutcome, err := invoiceRepo.SumByMonth(ctx, userID, refMonth)
	if err != nil {
		return fmt.Errorf("transactions/reconcile: soma card invoice: %w", err)
	}

	totalOutcome := outcome + cardOutcome

	current, getErr := summaryRepo.Get(ctx, userID, refMonth)
	if getErr != nil {
		return fmt.Errorf("transactions/reconcile: buscar summary: %w", getErr)
	}

	if current != nil {
		if current.IncomeCents() == income && current.OutcomeCents() == totalOutcome {
			return nil
		}
	}

	now := time.Now().UTC()
	if upsertErr := summaryRepo.Upsert(ctx, userID, refMonth, income, totalOutcome, now); upsertErr != nil {
		return fmt.Errorf("transactions/reconcile: upsert summary: %w", upsertErr)
	}

	return nil
}
