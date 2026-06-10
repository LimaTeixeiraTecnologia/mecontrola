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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

const maxDeliveredAlerts = 10

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
	expenses        interfaces.ExpenseRepository
	budgets         interfaces.BudgetRepository
	thresholdStates interfaces.ThresholdStateRepository
	alerts          interfaces.AlertRepository
	uow             uow.UnitOfWork[struct{}]
	o11y            observability.Observability
}

func NewEvaluateAlert(
	expenses interfaces.ExpenseRepository,
	budgets interfaces.BudgetRepository,
	thresholdStates interfaces.ThresholdStateRepository,
	alerts interfaces.AlertRepository,
	u uow.UnitOfWork[struct{}],
	o11y observability.Observability,
) *EvaluateAlert {
	return &EvaluateAlert{
		expenses:        expenses,
		budgets:         budgets,
		thresholdStates: thresholdStates,
		alerts:          alerts,
		uow:             u,
		o11y:            o11y,
	}
}

func (uc *EvaluateAlert) Execute(ctx context.Context, in EvaluateAlertInput) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "budgets.usecase.evaluate_alert")
	defer span.End()

	_, execErr := uc.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (struct{}, error) {
		sumByRoot, sumErr := uc.expenses.SumByRoot(ctx, tx, in.UserID, in.Competence)
		if sumErr != nil {
			return struct{}{}, fmt.Errorf("budgets.usecase.evaluate_alert: somar despesas: %w", sumErr)
		}

		budget, budgetErr := uc.budgets.GetByUserCompetence(ctx, tx, in.UserID, in.Competence)
		if budgetErr != nil {
			if errors.Is(budgetErr, interfaces.ErrBudgetNotFound) {
				uc.o11y.Logger().Warn(ctx, "budgets.usecase.evaluate_alert.suppressed",
					observability.String("reason", "budget_not_found"),
					observability.String("user_id", in.UserID.String()),
					observability.String("competence", in.Competence.String()),
				)
				return struct{}{}, nil
			}
			return struct{}{}, fmt.Errorf("budgets.usecase.evaluate_alert: buscar orçamento: %w", budgetErr)
		}

		if !budget.IsActive() {
			uc.o11y.Logger().Warn(ctx, "budgets.usecase.evaluate_alert.suppressed",
				observability.String("reason", "budget_not_active"),
				observability.String("user_id", in.UserID.String()),
				observability.String("competence", in.Competence.String()),
			)
			return struct{}{}, nil
		}

		spentCents := sumByRoot[in.RootSlug]

		var plannedCents int64
		for _, alloc := range budget.Allocations() {
			if alloc.RootSlug() == in.RootSlug {
				plannedCents = alloc.PlannedCents()
				break
			}
		}

		if plannedCents <= 0 {
			return struct{}{}, nil
		}

		currentlyCrossed, crossedErr := uc.thresholdStates.GetCurrentlyCrossed(ctx, tx, in.UserID, in.Competence, in.RootSlug)
		if crossedErr != nil {
			return struct{}{}, fmt.Errorf("budgets.usecase.evaluate_alert: obter estado dos limiares: %w", crossedErr)
		}

		transitions, evalErr := services.EvaluateThresholds(spentCents, plannedCents, currentlyCrossed)
		if evalErr != nil {
			return struct{}{}, fmt.Errorf("budgets.usecase.evaluate_alert: avaliar limiares: %w", evalErr)
		}

		now := time.Now().UTC()

		for _, t := range transitions {
			key := entities.ThresholdKey{
				UserID:     in.UserID,
				Competence: in.Competence,
				RootSlug:   in.RootSlug,
				Threshold:  t.Threshold,
			}

			transitioned, upsertErr := uc.thresholdStates.UpsertIfTransition(ctx, tx, key, t.NowCrossed, in.CommittedAt)
			if upsertErr != nil {
				return struct{}{}, fmt.Errorf("budgets.usecase.evaluate_alert: upsert estado: %w", upsertErr)
			}

			if !transitioned {
				if !t.NowCrossed && t.WasCrossed {
					uc.o11y.Logger().Warn(ctx, "budgets.usecase.evaluate_alert.suppressed_stale",
						observability.String("root_slug", in.RootSlug.String()),
						observability.String("threshold", t.Threshold.String()),
					)
				}
				continue
			}

			if !t.NowCrossed {
				continue
			}

			alertState := resolveAlertState(in.Competence, in.CutoffCompetenceBR)

			if alertState == entities.AlertStateSuppressedRetroactive {
				alert := entities.NewAlert(in.UserID, in.Competence, in.RootSlug, t.Threshold,
					entities.AlertStateSuppressedRetroactive, in.CommittedAt, spentCents, plannedCents, now)
				if insertErr := uc.alerts.Insert(ctx, tx, alert); insertErr != nil {
					return struct{}{}, fmt.Errorf("budgets.usecase.evaluate_alert: inserir alerta retroativo: %w", insertErr)
				}
				uc.o11y.Logger().Warn(ctx, "budgets.usecase.evaluate_alert.suppressed_retroactive",
					observability.String("root_slug", in.RootSlug.String()),
					observability.String("threshold", t.Threshold.String()),
				)
				continue
			}

			count, countErr := uc.alerts.CountDelivered(ctx, tx, key)
			if countErr != nil {
				return struct{}{}, fmt.Errorf("budgets.usecase.evaluate_alert: contar alertas: %w", countErr)
			}

			if count >= maxDeliveredAlerts {
				alert := entities.NewAlert(in.UserID, in.Competence, in.RootSlug, t.Threshold,
					entities.AlertStateRateLimited, in.CommittedAt, spentCents, plannedCents, now)
				if insertErr := uc.alerts.Insert(ctx, tx, alert); insertErr != nil {
					return struct{}{}, fmt.Errorf("budgets.usecase.evaluate_alert: inserir alerta rate_limited: %w", insertErr)
				}
				uc.o11y.Logger().Warn(ctx, "budgets.usecase.evaluate_alert.rate_limited",
					observability.String("root_slug", in.RootSlug.String()),
					observability.String("threshold", t.Threshold.String()),
				)
				continue
			}

			alert := entities.NewAlert(in.UserID, in.Competence, in.RootSlug, t.Threshold,
				entities.AlertStateDelivered, in.CommittedAt, spentCents, plannedCents, now)
			if insertErr := uc.alerts.Insert(ctx, tx, alert); insertErr != nil {
				return struct{}{}, fmt.Errorf("budgets.usecase.evaluate_alert: inserir alerta: %w", insertErr)
			}
		}

		return struct{}{}, nil
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

func resolveAlertState(expenseCompetence, cutoff valueobjects.Competence) entities.AlertState {
	if expenseCompetence.Before(cutoff) {
		return entities.AlertStateSuppressedRetroactive
	}
	return entities.AlertStateDelivered
}
