package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type EvaluateThresholdAlerts struct {
	factory    interfaces.RepositoryFactory
	publisher  interfaces.ThresholdAlertPublisher
	uow        uow.UnitOfWork
	thresholds services.ThresholdConfig
	location   *time.Location
	scanLimit  int
	o11y       observability.Observability
	dispatched observability.Counter
}

func NewEvaluateThresholdAlerts(
	factory interfaces.RepositoryFactory,
	publisher interfaces.ThresholdAlertPublisher,
	u uow.UnitOfWork,
	thresholds services.ThresholdConfig,
	location *time.Location,
	scanLimit int,
	o11y observability.Observability,
) *EvaluateThresholdAlerts {
	dispatched := o11y.Metrics().Counter(
		"budgets_threshold_alerts_dispatched_total",
		"Total de alertas proativos de limiar disparados",
		"1",
	)
	if scanLimit <= 0 {
		scanLimit = 500
	}
	return &EvaluateThresholdAlerts{
		factory:    factory,
		publisher:  publisher,
		uow:        u,
		thresholds: thresholds,
		location:   location,
		scanLimit:  scanLimit,
		o11y:       o11y,
		dispatched: dispatched,
	}
}

func (uc *EvaluateThresholdAlerts) Execute(ctx context.Context) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "budgets.usecase.evaluate_threshold_alerts")
	defer span.End()

	now := time.Now().UTC()
	loc := uc.location
	if loc == nil {
		loc = time.UTC
	}
	competence := valueobjects.CompetenceFromTime(now, loc)
	refDay := now.Truncate(24 * time.Hour)

	uc.o11y.Logger().Info(ctx, "budgets.usecase.evaluate_threshold_alerts.start",
		observability.String("competence", competence.String()),
		observability.String("ref_day", refDay.Format("2006-01-02")),
	)

	_, err := uow.Do(ctx, uc.uow, func(ctx context.Context, tx database.DBTX) (struct{}, error) {
		return struct{}{}, uc.executeInTx(ctx, tx, competence, refDay, now)
	})
	if err != nil {
		span.RecordError(err)
		uc.o11y.Logger().Warn(ctx, "budgets.usecase.evaluate_threshold_alerts.failed",
			observability.Error(err),
		)
		return err
	}
	return nil
}

func (uc *EvaluateThresholdAlerts) executeInTx(ctx context.Context, tx database.DBTX, competence valueobjects.Competence, refDay time.Time, now time.Time) error {
	sentRepo := uc.factory.ThresholdAlertSentRepository(tx)
	cardReader := uc.factory.CardThresholdReader(tx)

	active, err := sentRepo.ListActiveForThresholdScan(ctx, competence, uc.scanLimit)
	if err != nil {
		return fmt.Errorf("budgets.usecase.evaluate_threshold_alerts: listar budgets ativos: %w", err)
	}
	activeCards, err := cardReader.ListActiveCardsForThresholdScan(ctx, competence, uc.scanLimit)
	if err != nil {
		return fmt.Errorf("budgets.usecase.evaluate_threshold_alerts: listar cards ativos: %w", err)
	}
	if len(active) == 0 && len(activeCards) == 0 {
		return nil
	}

	sent, err := sentRepo.ListSentForDay(ctx, refDay)
	if err != nil {
		return fmt.Errorf("budgets.usecase.evaluate_threshold_alerts: listar enviados: %w", err)
	}

	alreadySent := make(map[services.ThresholdSentKey]struct{}, len(sent))
	for _, s := range sent {
		alreadySent[services.ThresholdSentKey{
			UserID:   s.UserID,
			BudgetID: s.BudgetID,
			Kind:     s.Kind,
			RefDay:   s.RefDay.UTC().Truncate(24 * time.Hour),
		}] = struct{}{}
	}

	snapshots := uc.buildSnapshots(active)
	snapshots = append(snapshots, uc.buildCardSnapshots(activeCards)...)
	alerts := services.ThresholdWorkflow{}.DecideAlerts(snapshots, uc.thresholds, alreadySent, refDay)

	for _, alert := range alerts {
		if err := uc.publishAndPersist(ctx, tx, sentRepo, alert, now); err != nil {
			return err
		}
	}
	return nil
}

func (uc *EvaluateThresholdAlerts) buildCardSnapshots(active []interfaces.ActiveCardForScan) []services.ActiveBudgetSnapshot {
	out := make([]services.ActiveBudgetSnapshot, 0, len(active))
	for _, a := range active {
		if a.LimitCents <= 0 {
			continue
		}
		out = append(out, services.ActiveBudgetSnapshot{
			UserID:       a.UserID,
			BudgetID:     a.CardID,
			Kind:         services.ThresholdAlertCardLimit,
			CategoryID:   uuid.Nil,
			CardID:       a.CardID,
			PlannedCents: a.LimitCents,
			SpentCents:   a.SpentCents,
		})
	}
	return out
}

func (uc *EvaluateThresholdAlerts) buildSnapshots(active []interfaces.ActiveBudgetForScan) []services.ActiveBudgetSnapshot {
	out := make([]services.ActiveBudgetSnapshot, 0, len(active))
	for _, a := range active {
		kind := services.ThresholdAlertCategory
		if a.RootSlug == valueobjects.RootSlugMetas {
			kind = services.ThresholdAlertGoal
		}
		out = append(out, services.ActiveBudgetSnapshot{
			UserID:       a.UserID,
			BudgetID:     a.BudgetID,
			Kind:         kind,
			CategoryID:   uuid.Nil,
			CardID:       uuid.Nil,
			RootSlug:     a.RootSlug,
			PlannedCents: a.PlannedCents,
			SpentCents:   a.SpentCents,
		})
	}
	return out
}

func (uc *EvaluateThresholdAlerts) publishAndPersist(
	ctx context.Context,
	tx database.DBTX,
	sentRepo interfaces.ThresholdAlertSentRepository,
	alert services.DomainAlert,
	now time.Time,
) error {
	if err := uc.publisher.Publish(ctx, tx, alert, now); err != nil {
		return fmt.Errorf("budgets.usecase.evaluate_threshold_alerts: publicar: %w", err)
	}
	if err := sentRepo.InsertSent(ctx, interfaces.ThresholdAlertSentRecord{
		UserID:   alert.UserID,
		BudgetID: alert.BudgetID,
		Kind:     alert.Kind,
		RefDay:   alert.RefDay,
		SentAt:   now,
	}); err != nil {
		return fmt.Errorf("budgets.usecase.evaluate_threshold_alerts: marcar enviado: %w", err)
	}
	uc.dispatched.Add(ctx, 1, observability.String("kind", alert.Kind.String()))
	uc.o11y.Logger().Info(ctx, "budgets.usecase.evaluate_threshold_alerts.dispatched",
		observability.String("kind", alert.Kind.String()),
		observability.String("budget_id", alert.BudgetID.String()),
		observability.String("root_slug", alert.RootSlug.String()),
		observability.Int64("percent_used_bps", int64(alert.PercentUsedBps)),
	)
	return nil
}
