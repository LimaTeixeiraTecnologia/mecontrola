package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type thresholdAlertSentRepository struct {
	db   database.DBTX
	o11y observability.Observability
}

func NewThresholdAlertSentRepository(o11y observability.Observability, db database.DBTX) interfaces.ThresholdAlertSentRepository {
	return &thresholdAlertSentRepository{db: db, o11y: o11y}
}

func (r *thresholdAlertSentRepository) ListActiveForThresholdScan(ctx context.Context, refMonth valueobjects.Competence, limit int) ([]interfaces.ActiveBudgetForScan, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "budgets.repository.threshold_alert_sent.list_active_for_scan")
	defer span.End()

	if limit <= 0 {
		limit = 500
	}

	const query = `
		SELECT b.user_id,
		       b.id        AS budget_id,
		       b.competence,
		       a.root_slug,
		       a.planned_cents,
		       COALESCE(e.spent_cents, 0) AS spent_cents
		  FROM mecontrola.budgets b
		  JOIN mecontrola.budgets_allocations a ON a.budget_id = b.id
		  LEFT JOIN (
		        SELECT user_id, competence, root_slug, SUM(amount_cents) AS spent_cents
		          FROM mecontrola.budgets_expenses
		         WHERE deleted_at IS NULL
		         GROUP BY user_id, competence, root_slug
		  ) e ON e.user_id = b.user_id AND e.competence = b.competence AND e.root_slug = a.root_slug
		 WHERE b.state      = $1
		   AND b.competence = $2
		   AND a.planned_cents > 0
		 ORDER BY b.user_id, b.id, a.root_slug
		 LIMIT $3
	`

	rows, err := r.db.QueryContext(ctx, query, int(entities.BudgetStateActive), refMonth.String(), limit)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("budgets/postgres: threshold_alert_sent.list_active_for_scan: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []interfaces.ActiveBudgetForScan
	for rows.Next() {
		var (
			userID       uuid.UUID
			budgetID     uuid.UUID
			competence   string
			rootSlugStr  string
			plannedCents int64
			spentCents   int64
		)
		if err := rows.Scan(&userID, &budgetID, &competence, &rootSlugStr, &plannedCents, &spentCents); err != nil {
			return nil, fmt.Errorf("budgets/postgres: threshold_alert_sent.list_active_for_scan scan: %w", err)
		}
		comp, compErr := valueobjects.NewCompetence(competence)
		if compErr != nil {
			return nil, fmt.Errorf("budgets/postgres: threshold_alert_sent.list_active_for_scan competence: %w", compErr)
		}
		root, rootErr := valueobjects.ParseRootSlug(rootSlugStr)
		if rootErr != nil {
			return nil, fmt.Errorf("budgets/postgres: threshold_alert_sent.list_active_for_scan root_slug: %w", rootErr)
		}
		out = append(out, interfaces.ActiveBudgetForScan{
			UserID:       userID,
			BudgetID:     budgetID,
			Competence:   comp,
			RootSlug:     root,
			PlannedCents: plannedCents,
			SpentCents:   spentCents,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("budgets/postgres: threshold_alert_sent.list_active_for_scan rows: %w", err)
	}
	return out, nil
}

func (r *thresholdAlertSentRepository) ListSentForDay(ctx context.Context, refDay time.Time) ([]interfaces.ThresholdAlertSentRecord, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "budgets.repository.threshold_alert_sent.list_sent_for_day")
	defer span.End()

	day := refDay.UTC().Truncate(24 * time.Hour)

	const query = `
		SELECT user_id, budget_id, kind, ref_day, sent_at
		  FROM mecontrola.budget_alerts_sent
		 WHERE ref_day = $1::date
	`

	rows, err := r.db.QueryContext(ctx, query, day)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("budgets/postgres: threshold_alert_sent.list_sent_for_day: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []interfaces.ThresholdAlertSentRecord
	for rows.Next() {
		var (
			userID   uuid.UUID
			budgetID uuid.UUID
			kindStr  string
			rd       time.Time
			sentAt   time.Time
		)
		if err := rows.Scan(&userID, &budgetID, &kindStr, &rd, &sentAt); err != nil {
			return nil, fmt.Errorf("budgets/postgres: threshold_alert_sent.list_sent_for_day scan: %w", err)
		}
		kind, kErr := parseAlertKind(kindStr)
		if kErr != nil {
			return nil, fmt.Errorf("budgets/postgres: threshold_alert_sent.list_sent_for_day kind: %w", kErr)
		}
		out = append(out, interfaces.ThresholdAlertSentRecord{
			UserID:   userID,
			BudgetID: budgetID,
			Kind:     kind,
			RefDay:   rd.UTC(),
			SentAt:   sentAt.UTC(),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("budgets/postgres: threshold_alert_sent.list_sent_for_day rows: %w", err)
	}
	return out, nil
}

func (r *thresholdAlertSentRepository) InsertSent(ctx context.Context, rec interfaces.ThresholdAlertSentRecord) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "budgets.repository.threshold_alert_sent.insert_sent")
	defer span.End()

	const query = `
		INSERT INTO mecontrola.budget_alerts_sent (user_id, budget_id, kind, ref_day, sent_at)
		VALUES ($1, $2, $3, $4::date, $5)
		ON CONFLICT (user_id, budget_id, kind, ref_day) DO NOTHING
	`

	day := rec.RefDay.UTC().Truncate(24 * time.Hour)
	_, err := r.db.ExecContext(ctx, query, rec.UserID, rec.BudgetID, rec.Kind.String(), day, rec.SentAt.UTC())
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("budgets/postgres: threshold_alert_sent.insert_sent: %w", err)
	}
	return nil
}

func (r *thresholdAlertSentRepository) IsNotified(ctx context.Context, userID, budgetID uuid.UUID, kind services.ThresholdAlertKind, refDay time.Time) (bool, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "budgets.repository.threshold_alert_sent.is_notified")
	defer span.End()

	day := refDay.UTC().Truncate(24 * time.Hour)
	const query = `
		SELECT notified_at IS NOT NULL
		  FROM mecontrola.budget_alerts_sent
		 WHERE user_id = $1 AND budget_id = $2 AND kind = $3 AND ref_day = $4::date
		 LIMIT 1
	`
	var notified bool
	err := r.db.QueryRowContext(ctx, query, userID, budgetID, kind.String(), day).Scan(&notified)
	if errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("budgets/postgres: threshold_alert_sent.is_notified: %w", interfaces.ErrAlertRecordMissing)
	}
	if err != nil {
		span.RecordError(err)
		return false, fmt.Errorf("budgets/postgres: threshold_alert_sent.is_notified: %w", err)
	}
	return notified, nil
}

func (r *thresholdAlertSentRepository) MarkNotified(ctx context.Context, userID, budgetID uuid.UUID, kind services.ThresholdAlertKind, refDay time.Time, channel string, notifiedAt time.Time) (bool, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "budgets.repository.threshold_alert_sent.mark_notified")
	defer span.End()

	day := refDay.UTC().Truncate(24 * time.Hour)
	const query = `
		UPDATE mecontrola.budget_alerts_sent
		   SET notified_at = $5,
		       notify_channel = $6
		 WHERE user_id = $1 AND budget_id = $2 AND kind = $3 AND ref_day = $4::date
		   AND notified_at IS NULL
	`
	result, err := r.db.ExecContext(ctx, query, userID, budgetID, kind.String(), day, notifiedAt.UTC(), channel)
	if err != nil {
		span.RecordError(err)
		return false, fmt.Errorf("budgets/postgres: threshold_alert_sent.mark_notified: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		span.RecordError(err)
		return false, fmt.Errorf("budgets/postgres: threshold_alert_sent.mark_notified rows affected: %w", err)
	}
	return rows > 0, nil
}

func parseAlertKind(s string) (services.ThresholdAlertKind, error) {
	switch s {
	case "category_threshold":
		return services.ThresholdAlertCategory, nil
	case "goal_achieved":
		return services.ThresholdAlertGoal, nil
	default:
		return 0, fmt.Errorf("budgets/postgres: kind desconhecido: %q", s)
	}
}
