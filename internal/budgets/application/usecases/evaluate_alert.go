package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/commands"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

var ErrEvaluateAlertInvalidPayload = errors.New("budgets: payload inválido para avaliação de alerta")

type EvaluateAlertInput struct {
	EventID            string
	Payload            json.RawMessage
	CommittedAt        time.Time
	CutoffCompetenceBR valueobjects.Competence
	UserID             uuid.UUID
	Competence         valueobjects.Competence
	RootSlug           valueobjects.RootSlug
}

type EvaluateAlert struct {
	factory  interfaces.RepositoryFactory
	uow      uow.UnitOfWork[struct{}]
	o11y     observability.Observability
	resolver *services.AlertStateResolver
}

func NewEvaluateAlert(
	factory interfaces.RepositoryFactory,
	u uow.UnitOfWork[struct{}],
	o11y observability.Observability,
) *EvaluateAlert {
	return &EvaluateAlert{
		factory:  factory,
		uow:      u,
		o11y:     o11y,
		resolver: services.NewAlertStateResolver(),
	}
}

func (uc *EvaluateAlert) Execute(ctx context.Context, in EvaluateAlertInput) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "budgets.usecase.evaluate_alert")
	defer span.End()

	if _, err := commands.NewEvaluateAlertCommand(in.UserID.String(), in.Competence.String(), time.Now().UTC()); err != nil {
		span.RecordError(err)
		return err
	}

	_, execErr := uc.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (struct{}, error) {
		return struct{}{}, uc.executeInTx(ctx, tx, in)
	})

	if execErr != nil {
		span.RecordError(execErr)
		uc.o11y.Logger().Warn(ctx, "budgets.usecase.evaluate_alert.failed",
			observability.String("user_id", in.UserID.String()),
			observability.String("competence", in.Competence.String()),
			observability.Error(execErr),
		)
		return execErr
	}

	return nil
}

func (uc *EvaluateAlert) executeInTx(ctx context.Context, tx database.DBTX, in EvaluateAlertInput) error {
	expenses := uc.factory.ExpenseRepository(tx)
	budgets := uc.factory.BudgetRepository(tx)
	thresholdStates := uc.factory.ThresholdStateRepository(tx)
	alerts := uc.factory.AlertRepository(tx)

	spentByRoot, budget, err := uc.loadBudgetContext(ctx, expenses, budgets, in)
	if err != nil || budget.ID() == uuid.Nil {
		return err
	}

	plannedCents := uc.plannedCents(budget, in.RootSlug)
	if plannedCents <= 0 {
		return nil
	}

	spentCents := spentByRoot[in.RootSlug]
	currentlyCrossed, err := thresholdStates.GetCurrentlyCrossed(ctx, in.UserID, in.Competence, in.RootSlug)
	if err != nil {
		return fmt.Errorf("budgets.usecase.evaluate_alert: obter estado dos limiares: %w", err)
	}

	transitions, err := services.EvaluateThresholds(spentCents, plannedCents, currentlyCrossed)
	if err != nil {
		return fmt.Errorf("budgets.usecase.evaluate_alert: avaliar limiares: %w", err)
	}

	for _, transition := range transitions {
		if err := uc.handleTransition(ctx, thresholdStates, alerts, in, transition, spentCents, plannedCents, time.Now().UTC()); err != nil {
			return err
		}
	}

	return nil
}

func (uc *EvaluateAlert) loadBudgetContext(ctx context.Context, expenses interfaces.ExpenseRepository, budgets interfaces.BudgetRepository, in EvaluateAlertInput) (map[valueobjects.RootSlug]int64, entities.Budget, error) {
	spentByRoot, err := expenses.SumByRoot(ctx, in.UserID, in.Competence)
	if err != nil {
		return nil, entities.Budget{}, fmt.Errorf("budgets.usecase.evaluate_alert: somar despesas: %w", err)
	}

	budget, err := budgets.GetByUserCompetence(ctx, in.UserID, in.Competence)
	if err != nil {
		if errors.Is(err, interfaces.ErrBudgetNotFound) {
			uc.logSuppressed(ctx, "budget_not_found", in)
			return nil, entities.Budget{}, nil
		}
		return nil, entities.Budget{}, fmt.Errorf("budgets.usecase.evaluate_alert: buscar orçamento: %w", err)
	}

	if !budget.IsActive() {
		uc.logSuppressed(ctx, "budget_not_active", in)
		return nil, entities.Budget{}, nil
	}

	return spentByRoot, budget, nil
}

func (uc *EvaluateAlert) plannedCents(budget entities.Budget, rootSlug valueobjects.RootSlug) int64 {
	for _, allocation := range budget.Allocations() {
		if allocation.RootSlug() == rootSlug {
			return allocation.PlannedCents()
		}
	}
	return 0
}

func (uc *EvaluateAlert) handleTransition(
	ctx context.Context,
	thresholdStates interfaces.ThresholdStateRepository,
	alerts interfaces.AlertRepository,
	in EvaluateAlertInput,
	transition services.Transition,
	spentCents int64,
	plannedCents int64,
	now time.Time,
) error {
	key := entities.ThresholdKey{
		UserID:     in.UserID,
		Competence: in.Competence,
		RootSlug:   in.RootSlug,
		Threshold:  transition.Threshold,
	}

	transitioned, err := thresholdStates.UpsertIfTransition(ctx, key, transition.NowCrossed, in.CommittedAt)
	if err != nil {
		return fmt.Errorf("budgets.usecase.evaluate_alert: upsert estado: %w", err)
	}
	if !transitioned {
		uc.logStaleTransition(ctx, in.RootSlug, transition)
		return nil
	}
	if !transition.NowCrossed {
		return nil
	}

	return uc.insertAlert(ctx, alerts, in, key, transition, spentCents, plannedCents, now)
}

func (uc *EvaluateAlert) insertAlert(
	ctx context.Context,
	alerts interfaces.AlertRepository,
	in EvaluateAlertInput,
	key entities.ThresholdKey,
	transition services.Transition,
	spentCents int64,
	plannedCents int64,
	now time.Time,
) error {
	if uc.resolver.Resolve(in.Competence, in.CutoffCompetenceBR, 0) == entities.AlertStateSuppressedRetroactive {
		return uc.insertAlertWithState(ctx, alerts, in, transition, spentCents, plannedCents, now, entities.AlertStateSuppressedRetroactive, "budgets.usecase.evaluate_alert.suppressed_retroactive", "inserir alerta retroativo")
	}

	count, err := alerts.CountDelivered(ctx, key)
	if err != nil {
		return fmt.Errorf("budgets.usecase.evaluate_alert: contar alertas: %w", err)
	}

	state := uc.resolver.Resolve(in.Competence, in.CutoffCompetenceBR, int(count))
	if state == entities.AlertStateRateLimited {
		return uc.insertAlertWithState(ctx, alerts, in, transition, spentCents, plannedCents, now, entities.AlertStateRateLimited, "budgets.usecase.evaluate_alert.rate_limited", "inserir alerta rate_limited")
	}

	return uc.insertAlertWithState(ctx, alerts, in, transition, spentCents, plannedCents, now, entities.AlertStateDelivered, "", "inserir alerta")
}

func (uc *EvaluateAlert) insertAlertWithState(
	ctx context.Context,
	alerts interfaces.AlertRepository,
	in EvaluateAlertInput,
	transition services.Transition,
	spentCents int64,
	plannedCents int64,
	now time.Time,
	state entities.AlertState,
	logKey string,
	errorContext string,
) error {
	alert := entities.NewAlert(in.UserID, in.Competence, in.RootSlug, transition.Threshold, state, in.CommittedAt, spentCents, plannedCents, now)
	if err := alerts.Insert(ctx, alert); err != nil {
		return fmt.Errorf("budgets.usecase.evaluate_alert: %s: %w", errorContext, err)
	}
	if logKey != "" {
		uc.o11y.Logger().Warn(ctx, logKey,
			observability.String("root_slug", in.RootSlug.String()),
			observability.String("threshold", transition.Threshold.String()),
		)
	}
	return nil
}

func (uc *EvaluateAlert) logSuppressed(ctx context.Context, reason string, in EvaluateAlertInput) {
	uc.o11y.Logger().Warn(ctx, "budgets.usecase.evaluate_alert.suppressed",
		observability.String("reason", reason),
		observability.String("user_id", in.UserID.String()),
		observability.String("competence", in.Competence.String()),
	)
}

func (uc *EvaluateAlert) logStaleTransition(ctx context.Context, rootSlug valueobjects.RootSlug, transition services.Transition) {
	if transition.NowCrossed || !transition.WasCrossed {
		return
	}
	uc.o11y.Logger().Warn(ctx, "budgets.usecase.evaluate_alert.suppressed_stale",
		observability.String("root_slug", rootSlug.String()),
		observability.String("threshold", transition.Threshold.String()),
	)
}
