package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type pendingEventRepository struct {
	db   database.DBTX
	o11y observability.Observability
}

func NewPendingEventRepository(o11y observability.Observability, db database.DBTX) interfaces.PendingEventRepository {
	return &pendingEventRepository{db: db, o11y: o11y}
}

func (r *pendingEventRepository) Insert(ctx context.Context, p entities.PendingEvent) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "budgets.repository.pending_event.insert")
	defer span.End()

	const query = `
		INSERT INTO mecontrola.budgets_expense_events_pending
		       (id, event_id, source, user_id, external_transaction_id,
		        expected_version, mutation_kind, payload, state, received_at,
		        transitioned_at, reason)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	_, err := r.db.ExecContext(ctx, query,
		p.ID(), p.EventID(), p.Source().String(), p.UserID(),
		p.ExternalTransactionID().String(),
		p.ExpectedVersion(), int(p.MutationKind()), p.Payload(),
		int(p.State()), p.ReceivedAt(), p.TransitionedAt(), nullableString(p.Reason()),
	)
	if err != nil {
		span.RecordError(err)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return fmt.Errorf("budgets/postgres: insert pending_event: %w", interfaces.ErrPendingEventDuplicate)
		}
		return fmt.Errorf("budgets/postgres: insert pending_event: %w", err)
	}
	return nil
}

func (r *pendingEventRepository) ListReady(ctx context.Context, limit int) ([]entities.PendingEvent, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "budgets.repository.pending_event.list_ready")
	defer span.End()

	if limit <= 0 {
		limit = 100
	}

	const query = `
		SELECT id, event_id, source, user_id, external_transaction_id,
		       expected_version, mutation_kind, payload, state, received_at,
		       transitioned_at, reason
		  FROM mecontrola.budgets_expense_events_pending
		 WHERE state = 1
		 ORDER BY received_at ASC
		 LIMIT $1
		   FOR UPDATE SKIP LOCKED
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("budgets/postgres: list_ready: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return r.scanPendingEvents(rows)
}

func (r *pendingEventRepository) Transition(ctx context.Context, id uuid.UUID, to entities.PendingState, reason string) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "budgets.repository.pending_event.transition")
	defer span.End()

	const query = `
		UPDATE mecontrola.budgets_expense_events_pending
		   SET state           = $1,
		       reason          = $2,
		       transitioned_at = $3
		 WHERE id = $4
	`

	now := time.Now().UTC()
	result, err := r.db.ExecContext(ctx, query, int(to), nullableString(reason), now, id)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("budgets/postgres: transition pending_event: %w", err)
	}

	affected, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return fmt.Errorf("budgets/postgres: transition rows_affected: %w", rowsErr)
	}
	if affected == 0 {
		return fmt.Errorf("budgets/postgres: transition: %w", interfaces.ErrPendingEventNotFound)
	}
	return nil
}

func (r *pendingEventRepository) scanPendingEvents(rows database.Rows) ([]entities.PendingEvent, error) {
	var result []entities.PendingEvent
	for rows.Next() {
		p, err := r.scanPendingEventRow(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("budgets/postgres: pending_event rows iteration: %w", err)
	}
	return result, nil
}

func (r *pendingEventRepository) scanPendingEventRow(s alertScanner) (entities.PendingEvent, error) {
	var (
		id                    uuid.UUID
		eventID               uuid.UUID
		sourceStr             string
		userID                uuid.UUID
		externalTransactionID string
		expectedVersion       int64
		mutationKindInt       int
		payload               []byte
		state                 int
		receivedAt            time.Time
		transitionedAt        sql.NullTime
		reason                sql.NullString
	)

	err := s.Scan(
		&id, &eventID, &sourceStr, &userID, &externalTransactionID,
		&expectedVersion, &mutationKindInt, &payload, &state, &receivedAt,
		&transitionedAt, &reason,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return entities.PendingEvent{}, fmt.Errorf("budgets/postgres: scan pending_event: %w", interfaces.ErrPendingEventNotFound)
	}
	if err != nil {
		return entities.PendingEvent{}, fmt.Errorf("budgets/postgres: scan pending_event: %w", err)
	}

	source, sourceErr := valueobjects.NewProducerSource(sourceStr)
	if sourceErr != nil {
		return entities.PendingEvent{}, fmt.Errorf("budgets/postgres: parse pending_event source: %w", sourceErr)
	}

	extID, extErr := valueobjects.NewExternalTransactionID(externalTransactionID)
	if extErr != nil {
		return entities.PendingEvent{}, fmt.Errorf("budgets/postgres: parse pending_event external_transaction_id: %w", extErr)
	}

	var transitionedAtPtr *time.Time
	if transitionedAt.Valid {
		t := transitionedAt.Time
		transitionedAtPtr = &t
	}

	reasonStr := ""
	if reason.Valid {
		reasonStr = reason.String
	}

	return entities.HydratePendingEvent(
		id, eventID, source, userID, extID,
		expectedVersion, valueobjects.MutationKind(mutationKindInt), payload,
		entities.PendingState(state), receivedAt, transitionedAtPtr, reasonStr,
	), nil
}

func nullableString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
