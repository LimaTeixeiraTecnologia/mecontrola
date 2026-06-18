package postgres

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

func visibleStatesClause(startIdx int) (string, []any, int) {
	states := entities.VisibleAlertStates()
	placeholders := make([]string, 0, len(states))
	args := make([]any, 0, len(states))
	idx := startIdx
	for _, st := range states {
		placeholders = append(placeholders, fmt.Sprintf("$%d", idx))
		args = append(args, int(st))
		idx++
	}
	return "state IN (" + strings.Join(placeholders, ", ") + ")", args, idx
}

type alertRepository struct {
	db   database.DBTX
	o11y observability.Observability
}

func NewAlertRepository(o11y observability.Observability, db database.DBTX) interfaces.AlertRepository {
	return &alertRepository{db: db, o11y: o11y}
}

func (r *alertRepository) Insert(ctx context.Context, a entities.Alert) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "budgets.repository.alert.insert")
	defer span.End()

	const query = `
		INSERT INTO mecontrola.budgets_alerts
		       (id, user_id, competence, root_slug, threshold, state,
		        triggered_by_committed_at, spent_cents, planned_cents, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err := r.db.ExecContext(ctx, query,
		a.ID(), a.UserID(), a.Competence().String(), a.RootSlug().String(),
		a.Threshold().Int(), int(a.State()),
		a.TriggeredByCommittedAt(), a.SpentCents(), a.PlannedCents(), a.CreatedAt(),
	)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("budgets/postgres: insert alert: %w", err)
	}
	return nil
}

func (r *alertRepository) CountDelivered(ctx context.Context, k entities.ThresholdKey) (int64, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "budgets.repository.alert.count_delivered")
	defer span.End()

	statesClause, stateArgs, _ := visibleStatesClause(5)
	query := `
		SELECT COUNT(*)
		  FROM mecontrola.budgets_alerts
		 WHERE user_id    = $1
		   AND competence = $2
		   AND root_slug  = $3
		   AND threshold  = $4
		   AND ` + statesClause

	args := append([]any{
		k.UserID, k.Competence.String(), k.RootSlug.String(), k.Threshold.Int(),
	}, stateArgs...)

	row := r.db.QueryRowContext(ctx, query, args...)

	var count int64
	if err := row.Scan(&count); err != nil {
		span.RecordError(err)
		return 0, fmt.Errorf("budgets/postgres: count_delivered: %w", err)
	}
	return count, nil
}

func (r *alertRepository) ListForUser(ctx context.Context, userID uuid.UUID, q input.AlertQuery) ([]entities.Alert, string, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "budgets.repository.alert.list_for_user")
	defer span.End()

	limit := q.Limit
	if limit <= 0 {
		limit = 20
	}

	listQuery, err := r.buildListForUserQuery(userID, q, limit)
	if err != nil {
		return nil, "", fmt.Errorf("budgets/postgres: list_for_user invalid cursor: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, listQuery.query, listQuery.args...)
	if err != nil {
		span.RecordError(err)
		return nil, "", fmt.Errorf("budgets/postgres: list_for_user: %w", err)
	}
	defer func() { _ = rows.Close() }()

	alerts, err := r.scanAlerts(rows)
	if err != nil {
		span.RecordError(err)
		return nil, "", err
	}

	var nextCursor string
	if len(alerts) > limit {
		alerts = alerts[:limit]
		last := alerts[len(alerts)-1]
		nextCursor = encodeCursor(last.CreatedAt(), last.ID())
	}

	return alerts, nextCursor, nil
}

type alertListQuery struct {
	query string
	args  []any
}

func (r *alertRepository) buildListForUserQuery(userID uuid.UUID, q input.AlertQuery, limit int) (alertListQuery, error) {
	cursorTime, cursorID, err := decodeCursor(q.Cursor)
	if err != nil {
		return alertListQuery{}, err
	}

	statesClause, stateArgs, nextIdx := visibleStatesClause(2)
	query := `
		SELECT id, user_id, competence, root_slug, threshold, state,
		       triggered_by_committed_at, spent_cents, planned_cents, created_at
		  FROM mecontrola.budgets_alerts
		 WHERE user_id = $1
		   AND ` + statesClause + `
	`
	args := append([]any{userID}, stateArgs...)
	index := nextIdx

	query, args, index = r.appendListForUserFilters(query, args, index, q)
	if !cursorTime.IsZero() {
		args = append(args, cursorTime, cursorID)
		query += fmt.Sprintf(" AND (created_at, id) < ($%d, $%d)", index, index+1)
		index += 2
	}

	args = append(args, limit+1)
	query += fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT $%d", index)
	return alertListQuery{query: query, args: args}, nil
}

func (r *alertRepository) appendListForUserFilters(query string, args []any, index int, q input.AlertQuery) (string, []any, int) {
	if q.Competence != nil {
		args = append(args, q.Competence.String())
		query += fmt.Sprintf(" AND competence = $%d", index)
		index++
	}
	if q.RootSlug != nil {
		args = append(args, q.RootSlug.String())
		query += fmt.Sprintf(" AND root_slug = $%d", index)
		index++
	}
	if q.Threshold != nil {
		args = append(args, q.Threshold.Int())
		query += fmt.Sprintf(" AND threshold = $%d", index)
		index++
	}
	return query, args, index
}

func (r *alertRepository) scanAlerts(rows *sql.Rows) ([]entities.Alert, error) {
	var result []entities.Alert
	for rows.Next() {
		a, err := r.scanAlertRow(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("budgets/postgres: alert rows iteration: %w", err)
	}
	return result, nil
}

type alertScanner interface {
	Scan(dest ...any) error
}

func (r *alertRepository) scanAlertRow(s alertScanner) (entities.Alert, error) {
	var (
		id                     uuid.UUID
		userID                 uuid.UUID
		competenceStr          string
		rootSlugStr            string
		thresholdInt           int
		state                  int
		triggeredByCommittedAt time.Time
		spentCents             int64
		plannedCents           int64
		createdAt              time.Time
	)

	err := s.Scan(
		&id, &userID, &competenceStr, &rootSlugStr, &thresholdInt, &state,
		&triggeredByCommittedAt, &spentCents, &plannedCents, &createdAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return entities.Alert{}, fmt.Errorf("budgets/postgres: scan alert: %w", interfaces.ErrExpenseNotFound)
	}
	if err != nil {
		return entities.Alert{}, fmt.Errorf("budgets/postgres: scan alert: %w", err)
	}

	competence, compErr := valueobjects.NewCompetence(competenceStr)
	if compErr != nil {
		return entities.Alert{}, fmt.Errorf("budgets/postgres: parse alert competence: %w", compErr)
	}

	rootSlug, slugErr := valueobjects.ParseRootSlug(rootSlugStr)
	if slugErr != nil {
		return entities.Alert{}, fmt.Errorf("budgets/postgres: parse alert root_slug: %w", slugErr)
	}

	threshold, threshErr := valueobjects.ParseThreshold(thresholdInt)
	if threshErr != nil {
		return entities.Alert{}, fmt.Errorf("budgets/postgres: parse alert threshold: %w", threshErr)
	}

	return entities.HydrateAlert(
		id, userID, competence, rootSlug, threshold,
		entities.AlertState(state), triggeredByCommittedAt,
		spentCents, plannedCents, createdAt,
	), nil
}

func encodeCursor(t time.Time, id uuid.UUID) string {
	raw := fmt.Sprintf("%s|%s", t.UTC().Format(time.RFC3339Nano), id.String())
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(cursor string) (time.Time, uuid.UUID, error) {
	if cursor == "" {
		return time.Time{}, uuid.Nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("decode cursor: %w", err)
	}
	parts := splitCursor(string(raw))
	if len(parts) != 2 {
		return time.Time{}, uuid.Nil, fmt.Errorf("parse cursor format: expected 2 parts got %d", len(parts))
	}
	t, parseErr := time.Parse(time.RFC3339Nano, parts[0])
	if parseErr != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("parse cursor time: %w", parseErr)
	}
	id, idErr := uuid.Parse(parts[1])
	if idErr != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("parse cursor id: %w", idErr)
	}
	return t, id, nil
}

func splitCursor(s string) []string {
	const uuidLen = 36
	idx := len(s) - uuidLen
	if idx <= 1 {
		return nil
	}
	if s[idx-1] != '|' {
		return nil
	}
	return []string{s[:idx-1], s[idx:]}
}

func (r *alertRepository) PurgeOld(ctx context.Context, olderThan string, limit int) (int64, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "budgets.repository.alert.purge_old")
	defer span.End()

	const query = `
		DELETE FROM mecontrola.budgets_alerts
		 WHERE id IN (
		   SELECT id FROM mecontrola.budgets_alerts
		    WHERE created_at < now() - $1::interval
		    LIMIT $2
		 )
	`

	result, err := r.db.ExecContext(ctx, query, olderThan, limit)
	if err != nil {
		span.RecordError(err)
		return 0, fmt.Errorf("budgets/postgres: purge_old_alerts: %w", err)
	}
	n, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return 0, fmt.Errorf("budgets/postgres: purge_old_alerts rows_affected: %w", rowsErr)
	}
	return n, nil
}
