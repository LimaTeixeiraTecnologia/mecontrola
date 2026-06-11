package usecases

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
)

const _retentionInterval = "24 months"

type PurgeRetention struct {
	factory   interfaces.RepositoryFactory
	uow       uow.UnitOfWork[struct{}]
	batchSize int
	o11y      observability.Observability
	purged    observability.Counter
	deferred  observability.Counter
}

func NewPurgeRetention(
	factory interfaces.RepositoryFactory,
	u uow.UnitOfWork[struct{}],
	batchSize int,
	o11y observability.Observability,
) *PurgeRetention {
	purged := o11y.Metrics().Counter(
		"budgets_retention_purged_total",
		"Total de registros purgados pelo job de retenção",
		"1",
	)
	deferred := o11y.Metrics().Counter(
		"budgets_retention_purge_deferred_total",
		"Total de expurgos adiados pelo job de retenção",
		"1",
	)
	return &PurgeRetention{
		factory:   factory,
		uow:       u,
		batchSize: batchSize,
		o11y:      o11y,
		purged:    purged,
		deferred:  deferred,
	}
}

func (uc *PurgeRetention) Execute(ctx context.Context) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "budgets.usecase.purge_retention")
	defer span.End()

	uc.o11y.Logger().Info(ctx, "budgets.usecase.purge_retention.start")

	hasPending, checkErr := uc.hasPendingNonTerminal(ctx)
	if checkErr != nil {
		span.RecordError(checkErr)
		return checkErr
	}

	if hasPending {
		uc.deferred.Add(ctx, 1, observability.String("reason", "pending_non_terminal"))
		uc.o11y.Logger().Warn(ctx, "budgets.usecase.purge_retention.deferred",
			observability.String("reason", "pending_non_terminal"),
		)
		return nil
	}

	if err := uc.purgeExpenses(ctx); err != nil {
		span.RecordError(err)
		return err
	}

	if err := uc.purgeAlerts(ctx); err != nil {
		span.RecordError(err)
		return err
	}

	uc.o11y.Logger().Info(ctx, "budgets.usecase.purge_retention.done")
	return nil
}

func (uc *PurgeRetention) hasPendingNonTerminal(ctx context.Context) (bool, error) {
	var found bool
	_, err := uc.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (struct{}, error) {
		pending := uc.factory.PendingEventRepository(tx)
		ready, listErr := pending.ListReady(ctx, 1)
		if listErr != nil {
			return struct{}{}, fmt.Errorf("budgets.usecase.purge_retention: verificar pendentes: %w", listErr)
		}
		found = len(ready) > 0
		return struct{}{}, nil
	})
	if err != nil {
		return false, err
	}
	return found, nil
}

func (uc *PurgeRetention) purgeExpenses(ctx context.Context) error {
	_, err := uc.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (struct{}, error) {
		expenses := uc.factory.ExpenseRepository(tx)
		n, purgeErr := expenses.PurgeDeleted(ctx, _retentionInterval, uc.batchSize)
		if purgeErr != nil {
			return struct{}{}, fmt.Errorf("budgets.usecase.purge_retention: purgar despesas: %w", purgeErr)
		}
		if n > 0 {
			uc.purged.Add(ctx, n, observability.String("table", "budgets_expenses"))
			uc.o11y.Logger().Info(ctx, "budgets.usecase.purge_retention.expenses_purged",
				observability.Int64("count", n),
			)
		}
		return struct{}{}, nil
	})
	return err
}

func (uc *PurgeRetention) purgeAlerts(ctx context.Context) error {
	_, err := uc.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (struct{}, error) {
		alerts := uc.factory.AlertRepository(tx)
		n, purgeErr := alerts.PurgeOld(ctx, _retentionInterval, uc.batchSize)
		if purgeErr != nil {
			return struct{}{}, fmt.Errorf("budgets.usecase.purge_retention: purgar alertas: %w", purgeErr)
		}
		if n > 0 {
			uc.purged.Add(ctx, n, observability.String("table", "budgets_alerts"))
			uc.o11y.Logger().Info(ctx, "budgets.usecase.purge_retention.alerts_purged",
				observability.Int64("count", n),
			)
		}
		return struct{}{}, nil
	})
	return err
}
