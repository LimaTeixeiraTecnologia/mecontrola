package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

const _abandonedDraftBatchSize = 200

type SignalAbandonedDrafts struct {
	factory  interfaces.RepositoryFactory
	uow      uow.UnitOfWork
	loc      *time.Location
	o11y     observability.Observability
	signaled observability.Counter
}

func NewSignalAbandonedDrafts(
	factory interfaces.RepositoryFactory,
	u uow.UnitOfWork,
	loc *time.Location,
	o11y observability.Observability,
) *SignalAbandonedDrafts {
	signaled := o11y.Metrics().Counter(
		"budgets_abandoned_drafts_total",
		"Total de rascunhos abandonados sinalizados pelo job",
		"1",
	)
	return &SignalAbandonedDrafts{
		factory:  factory,
		uow:      u,
		loc:      loc,
		o11y:     o11y,
		signaled: signaled,
	}
}

func (uc *SignalAbandonedDrafts) Execute(ctx context.Context) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "budgets.usecase.signal_abandoned_drafts")
	defer span.End()

	currentComp := valueobjects.CompetenceFromTime(time.Now().UTC(), uc.loc)

	uc.o11y.Logger().Info(ctx, "budgets.usecase.signal_abandoned_drafts.start",
		observability.String("current_competence", currentComp.String()),
	)

	drafts, listErr := uc.listAbandoned(ctx, currentComp)
	if listErr != nil {
		span.RecordError(listErr)
		return listErr
	}

	for _, d := range drafts {
		if err := uc.processOne(ctx, d); err != nil {
			uc.o11y.Logger().Warn(ctx, "budgets.usecase.signal_abandoned_drafts.process_failed",
				observability.String("budget_id", d.ID().String()),
				observability.Error(err),
			)
		}
	}

	return nil
}

func (uc *SignalAbandonedDrafts) listAbandoned(ctx context.Context, before valueobjects.Competence) ([]entities.Budget, error) {
	var result []entities.Budget
	_, err := uow.Do(ctx, uc.uow, func(ctx context.Context, tx database.DBTX) (struct{}, error) {
		budgets := uc.factory.BudgetRepository(tx)
		drafts, listErr := budgets.ListAbandonedDrafts(ctx, before, _abandonedDraftBatchSize)
		if listErr != nil {
			return struct{}{}, fmt.Errorf("budgets.usecase.signal_abandoned_drafts: listar rascunhos: %w", listErr)
		}
		result = drafts
		return struct{}{}, nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (uc *SignalAbandonedDrafts) processOne(ctx context.Context, draft entities.Budget) error {
	_, err := uow.Do(ctx, uc.uow, func(ctx context.Context, tx database.DBTX) (struct{}, error) {
		budgets := uc.factory.BudgetRepository(tx)
		already, checkErr := budgets.IsSignaledAbandoned(ctx, draft.ID())
		if checkErr != nil {
			return struct{}{}, fmt.Errorf("budgets.usecase.signal_abandoned_drafts: verificar sinal: %w", checkErr)
		}
		if already {
			return struct{}{}, nil
		}
		if signErr := budgets.SignalAbandoned(ctx, draft.ID()); signErr != nil {
			return struct{}{}, fmt.Errorf("budgets.usecase.signal_abandoned_drafts: sinalizar: %w", signErr)
		}
		uc.signaled.Add(ctx, 1,
			observability.String("competence", draft.Competence().String()),
		)
		uc.o11y.Logger().Info(ctx, "budgets.usecase.signal_abandoned_drafts.signaled",
			observability.String("budget_id", draft.ID().String()),
			observability.String("competence", draft.Competence().String()),
		)
		return struct{}{}, nil
	})
	return err
}
