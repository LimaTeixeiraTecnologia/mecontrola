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

const (
	EvaluateOutcomeEligible = "eligible"
	EvaluateOutcomeQueued   = "queued"
	EvaluateOutcomeSkipped  = "skipped"
)

type EvaluateThresholdAlerts struct {
	factory    interfaces.RepositoryFactory
	publisher  interfaces.ThresholdAlertPublisher
	uow        uow.UnitOfWork
	thresholds services.ThresholdConfig
	location   *time.Location
	scanLimit  int
	dryRun     bool
	o11y       observability.Observability
	dispatched observability.Counter
	evaluated  observability.Counter
	suppressed observability.Counter
	queued     observability.Counter
	dryRunHits observability.Counter
}

func NewEvaluateThresholdAlerts(
	factory interfaces.RepositoryFactory,
	publisher interfaces.ThresholdAlertPublisher,
	u uow.UnitOfWork,
	thresholds services.ThresholdConfig,
	location *time.Location,
	scanLimit int,
	dryRun bool,
	o11y observability.Observability,
) *EvaluateThresholdAlerts {
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
		dryRun:     dryRun,
		o11y:       o11y,
		dispatched: o11y.Metrics().Counter(
			"budgets_threshold_alerts_dispatched_total",
			"Total de alertas proativos de limiar disparados",
			"1",
		),
		evaluated: o11y.Metrics().Counter(
			"proactive_alerts_evaluated_total",
			"Total de alertas proativos avaliados por kind e outcome",
			"1",
		),
		suppressed: o11y.Metrics().Counter(
			"proactive_alerts_suppressed_total",
			"Total de alertas proativos suprimidos por kind e motivo",
			"1",
		),
		queued: o11y.Metrics().Counter(
			"proactive_alerts_queued_total",
			"Total de alertas proativos enfileirados para envio por kind",
			"1",
		),
		dryRunHits: o11y.Metrics().Counter(
			"proactive_alerts_dry_run_total",
			"Total de alertas proativos avaliados em dry-run por kind e outcome",
			"1",
		),
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
		observability.Bool("dry_run", uc.dryRun),
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

	active, err := sentRepo.ListActiveForThresholdScan(ctx, competence, uc.scanLimit)
	if err != nil {
		return fmt.Errorf("budgets.usecase.evaluate_threshold_alerts: listar budgets ativos: %w", err)
	}

	missing, err := sentRepo.ListUsersMissingBudget(ctx, competence, uc.scanLimit)
	if err != nil {
		return fmt.Errorf("budgets.usecase.evaluate_threshold_alerts: listar orcamentos ausentes: %w", err)
	}

	unreviewed, err := sentRepo.ListUnreviewedBudgets(ctx, competence, uc.scanLimit)
	if err != nil {
		return fmt.Errorf("budgets.usecase.evaluate_threshold_alerts: listar orcamentos nao revisados: %w", err)
	}

	if len(active) == 0 && len(missing) == 0 && len(unreviewed) == 0 {
		return nil
	}

	sent, err := sentRepo.ListSentForDay(ctx, refDay)
	if err != nil {
		return fmt.Errorf("budgets.usecase.evaluate_threshold_alerts: listar enviados: %w", err)
	}

	alreadySent := make(map[services.ThresholdSentKey]struct{}, len(sent))
	alreadyAlertedToday := make(map[uuid.UUID]struct{}, len(sent))
	for _, s := range sent {
		alreadySent[services.ThresholdSentKey{
			UserID:   s.UserID,
			BudgetID: s.BudgetID,
			Kind:     s.Kind,
			RefDay:   s.RefDay.UTC().Truncate(24 * time.Hour),
		}] = struct{}{}
		alreadyAlertedToday[s.UserID] = struct{}{}
	}

	candidates := services.ThresholdWorkflow{}.DecideAlerts(uc.buildSnapshots(active), uc.thresholds, alreadySent, refDay)
	candidates = append(candidates, services.DecideBudgetLifecycleAlerts(
		uc.buildMissingSnapshots(missing),
		uc.buildUnreviewedSnapshots(unreviewed),
		alreadySent,
		refDay,
	)...)

	for _, candidate := range candidates {
		uc.evaluated.Add(ctx, 1,
			observability.String("kind", candidate.Kind.String()),
			observability.String("outcome", EvaluateOutcomeEligible),
		)
	}

	selected, dropped := services.DecideDailyRoundAlerts(candidates, alreadyAlertedToday)

	for _, drop := range dropped {
		uc.suppressed.Add(ctx, 1,
			observability.String("kind", drop.Kind.String()),
			observability.String("reason", drop.Reason.String()),
		)
		uc.o11y.Logger().Info(ctx, "budgets.usecase.evaluate_threshold_alerts.suppressed",
			observability.String("kind", drop.Kind.String()),
			observability.String("reason", drop.Reason.String()),
		)
	}

	for _, alert := range selected {
		if uc.dryRun {
			uc.dryRunHits.Add(ctx, 1,
				observability.String("kind", alert.Kind.String()),
				observability.String("outcome", EvaluateOutcomeSkipped),
			)
			uc.o11y.Logger().Info(ctx, "budgets.usecase.evaluate_threshold_alerts.dry_run",
				observability.String("kind", alert.Kind.String()),
				observability.String("budget_id", alert.BudgetID.String()),
			)
			continue
		}
		if err := uc.publishAndPersist(ctx, tx, sentRepo, alert, now); err != nil {
			return err
		}
	}
	return nil
}

func (uc *EvaluateThresholdAlerts) buildSnapshots(active []interfaces.ActiveBudgetForScan) []services.ActiveBudgetSnapshot {
	out := make([]services.ActiveBudgetSnapshot, 0, len(active))
	for _, a := range active {
		out = append(out, services.ActiveBudgetSnapshot{
			UserID:       a.UserID,
			BudgetID:     a.BudgetID,
			CategoryID:   uuid.Nil,
			CardID:       uuid.Nil,
			RootSlug:     a.RootSlug,
			PlannedCents: a.PlannedCents,
			SpentCents:   a.SpentCents,
		})
	}
	return out
}

func (uc *EvaluateThresholdAlerts) buildMissingSnapshots(missing []interfaces.MissingBudgetForScan) []services.MissingBudgetSnapshot {
	out := make([]services.MissingBudgetSnapshot, 0, len(missing))
	for _, m := range missing {
		out = append(out, services.MissingBudgetSnapshot{UserID: m.UserID})
	}
	return out
}

func (uc *EvaluateThresholdAlerts) buildUnreviewedSnapshots(unreviewed []interfaces.UnreviewedBudgetForScan) []services.UnreviewedBudgetSnapshot {
	out := make([]services.UnreviewedBudgetSnapshot, 0, len(unreviewed))
	for _, u := range unreviewed {
		out = append(out, services.UnreviewedBudgetSnapshot{UserID: u.UserID, BudgetID: u.BudgetID})
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
	uc.queued.Add(ctx, 1, observability.String("kind", alert.Kind.String()))
	uc.evaluated.Add(ctx, 1,
		observability.String("kind", alert.Kind.String()),
		observability.String("outcome", EvaluateOutcomeQueued),
	)
	uc.o11y.Logger().Info(ctx, "budgets.usecase.evaluate_threshold_alerts.dispatched",
		observability.String("kind", alert.Kind.String()),
		observability.String("budget_id", alert.BudgetID.String()),
		observability.String("root_slug", alert.RootSlug.String()),
		observability.Int64("percent_used_bps", int64(alert.PercentUsedBps)),
	)
	return nil
}
