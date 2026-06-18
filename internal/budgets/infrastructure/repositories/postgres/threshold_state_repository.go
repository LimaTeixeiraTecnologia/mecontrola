package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type thresholdStateRepository struct {
	db   database.DBTX
	o11y observability.Observability
}

func NewThresholdStateRepository(o11y observability.Observability, db database.DBTX) interfaces.ThresholdStateRepository {
	return &thresholdStateRepository{db: db, o11y: o11y}
}

func (r *thresholdStateRepository) UpsertIfTransition(ctx context.Context, k entities.ThresholdKey, nowCrossed bool, committedAt time.Time) (bool, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "budgets.repository.threshold_state.upsert_if_transition")
	defer span.End()

	const query = `
		WITH current AS (
		    SELECT currently_crossed, last_evaluated_committed_at
		      FROM mecontrola.budgets_threshold_states
		     WHERE user_id   = $1
		       AND competence = $2
		       AND root_slug  = $3
		       AND threshold  = $4
		),
		upserted AS (
		    INSERT INTO mecontrola.budgets_threshold_states
		           (user_id, competence, root_slug, threshold,
		            currently_crossed, version,
		            last_crossed_at, last_uncrossed_at, last_evaluated_committed_at)
		    VALUES ($1, $2, $3, $4, $5, 0, $7, $8, $6)
		    ON CONFLICT (user_id, competence, root_slug, threshold) DO UPDATE
		       SET currently_crossed           =
		               CASE
		                   WHEN budgets_threshold_states.last_evaluated_committed_at IS NOT NULL
		                    AND $6 <= budgets_threshold_states.last_evaluated_committed_at
		                   THEN budgets_threshold_states.currently_crossed
		                   ELSE EXCLUDED.currently_crossed
		               END,
		           version                     =
		               CASE
		                   WHEN budgets_threshold_states.last_evaluated_committed_at IS NOT NULL
		                    AND $6 <= budgets_threshold_states.last_evaluated_committed_at
		                   THEN budgets_threshold_states.version
		                   WHEN budgets_threshold_states.currently_crossed <> EXCLUDED.currently_crossed
		                   THEN budgets_threshold_states.version + 1
		                   ELSE budgets_threshold_states.version
		               END,
		           last_crossed_at             =
		               CASE
		                   WHEN budgets_threshold_states.last_evaluated_committed_at IS NOT NULL
		                    AND $6 <= budgets_threshold_states.last_evaluated_committed_at
		                   THEN budgets_threshold_states.last_crossed_at
		                   WHEN EXCLUDED.currently_crossed = TRUE
		                   THEN $6
		                   ELSE budgets_threshold_states.last_crossed_at
		               END,
		           last_uncrossed_at           =
		               CASE
		                   WHEN budgets_threshold_states.last_evaluated_committed_at IS NOT NULL
		                    AND $6 <= budgets_threshold_states.last_evaluated_committed_at
		                   THEN budgets_threshold_states.last_uncrossed_at
		                   WHEN EXCLUDED.currently_crossed = FALSE
		                    AND budgets_threshold_states.currently_crossed = TRUE
		                   THEN $6
		                   ELSE budgets_threshold_states.last_uncrossed_at
		               END,
		           last_evaluated_committed_at =
		               CASE
		                   WHEN budgets_threshold_states.last_evaluated_committed_at IS NOT NULL
		                    AND $6 <= budgets_threshold_states.last_evaluated_committed_at
		                   THEN budgets_threshold_states.last_evaluated_committed_at
		                   ELSE $6
		               END
		    RETURNING currently_crossed, version
		),
		before AS (SELECT currently_crossed FROM current)
		SELECT upserted.version,
		       (SELECT currently_crossed FROM before) AS before_crossed,
		       upserted.currently_crossed             AS after_crossed
		  FROM upserted
	`

	var crossedAfterTS *time.Time
	var uncrossedAfterTS *time.Time
	if nowCrossed {
		t := committedAt
		crossedAfterTS = &t
	} else {
		t := committedAt
		uncrossedAfterTS = &t
	}

	row := r.db.QueryRowContext(ctx, query,
		k.UserID, k.Competence.String(), k.RootSlug.String(), k.Threshold.Int(),
		nowCrossed, committedAt,
		crossedAfterTS, uncrossedAfterTS,
	)

	var (
		version       int64
		beforeCrossed sql.NullBool
		afterCrossed  bool
	)

	if err := row.Scan(&version, &beforeCrossed, &afterCrossed); err != nil {
		span.RecordError(err)
		return false, fmt.Errorf("budgets/postgres: upsert_if_transition: %w", err)
	}

	if !beforeCrossed.Valid {
		return true, nil
	}
	return beforeCrossed.Bool != afterCrossed, nil
}

func (r *thresholdStateRepository) GetCurrentlyCrossed(ctx context.Context, userID uuid.UUID, competence valueobjects.Competence, rootSlug valueobjects.RootSlug) (map[valueobjects.Threshold]bool, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "budgets.repository.threshold_state.get_currently_crossed")
	defer span.End()

	const query = `
		SELECT threshold, currently_crossed
		  FROM mecontrola.budgets_threshold_states
		 WHERE user_id    = $1
		   AND competence = $2
		   AND root_slug  = $3
	`

	result := map[valueobjects.Threshold]bool{
		valueobjects.Threshold80:  false,
		valueobjects.Threshold100: false,
	}

	rows, err := r.db.QueryContext(ctx, query, userID, competence.String(), rootSlug.String())
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("budgets/postgres: get_currently_crossed: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var rawThreshold int
		var crossed bool
		if scanErr := rows.Scan(&rawThreshold, &crossed); scanErr != nil {
			span.RecordError(scanErr)
			return nil, fmt.Errorf("budgets/postgres: get_currently_crossed scan: %w", scanErr)
		}
		t, parseErr := valueobjects.ParseThreshold(rawThreshold)
		if parseErr != nil {
			continue
		}
		result[t] = crossed
	}

	if err := rows.Err(); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("budgets/postgres: get_currently_crossed rows: %w", err)
	}

	return result, nil
}
