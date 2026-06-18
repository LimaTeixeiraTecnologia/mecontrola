package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

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
	factory interfaces.RepositoryFactory
	uow     uow.UnitOfWork
	o11y    observability.Observability
}

func NewEvaluateAlert(
	factory interfaces.RepositoryFactory,
	u uow.UnitOfWork,
	o11y observability.Observability,
) *EvaluateAlert {
	return &EvaluateAlert{
		factory: factory,
		uow:     u,
		o11y:    o11y,
	}
}

func (uc *EvaluateAlert) Execute(ctx context.Context, in EvaluateAlertInput) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "budgets.usecase.evaluate_alert")
	defer span.End()

	if _, err := commands.NewEvaluateAlertCommand(in.UserID.String(), in.Competence.String(), time.Now().UTC()); err != nil {
		span.RecordError(err)
		return err
	}

	_, execErr := uow.Do(ctx, uc.uow, func(ctx context.Context, tx database.DBTX) (struct{}, error) {
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

	transitions, err := services.ThresholdEvaluator{}.EvaluateThresholds(spentCents, plannedCents, currentlyCrossed)
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
	isRetroactive := services.AlertWorkflow{}.IsRetroactiveAlert(in.Competence, in.CutoffCompetenceBR)

	if isRetroactive {
		decision := services.AlertWorkflow{}.DecideAlertForInsert(true, 0)
		return uc.persistDecidedAlert(ctx, alerts, in, transition, spentCents, plannedCents, now, decision)
	}

	count, err := alerts.CountDelivered(ctx, key)
	if err != nil {
		return fmt.Errorf("budgets.usecase.evaluate_alert: contar alertas: %w", err)
	}

	decision := services.AlertWorkflow{}.DecideAlertForInsert(false, int(count))
	return uc.persistDecidedAlert(ctx, alerts, in, transition, spentCents, plannedCents, now, decision)
}

func (uc *EvaluateAlert) persistDecidedAlert(
	ctx context.Context,
	alerts interfaces.AlertRepository,
	in EvaluateAlertInput,
	transition services.Transition,
	spentCents int64,
	plannedCents int64,
	now time.Time,
	decision services.AlertDecision,
) error {
	alert := entities.NewAlert(in.UserID, in.Competence, in.RootSlug, transition.Threshold, decision.State, in.CommittedAt, spentCents, plannedCents, now)
	if err := alerts.Insert(ctx, alert); err != nil {
		return fmt.Errorf("budgets.usecase.evaluate_alert: %s: %w", decision.ErrorContext, err)
	}
	if decision.LogKey != "" {
		uc.o11y.Logger().Warn(ctx, decision.LogKey,
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
