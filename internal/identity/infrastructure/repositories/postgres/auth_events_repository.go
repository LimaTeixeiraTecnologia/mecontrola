package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
)

const prefixAuthEventsRepository = "identity.repository.auth_events:"

type authEventsRepository struct {
	o11y observability.Observability
	db   database.DBTX
}

func NewAuthEventsRepository(o11y observability.Observability, db database.DBTX) interfaces.AuthEventsRepository {
	return &authEventsRepository{o11y: o11y, db: db}
}

func (r *authEventsRepository) Insert(ctx context.Context, event entities.AuthEvent) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "identity.repository.auth_events.insert")
	defer span.End()

	const query = `
		INSERT INTO auth_events (id, occurred_at, user_id, kind, source, reason, request_id, client_ip)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO NOTHING
	`

	var userID *uuid.UUID
	if id := event.UserID(); id != nil {
		userID = id
	}

	var reason sql.NullString
	if r := event.Reason(); r != nil {
		reason = sql.NullString{String: string(*r), Valid: true}
	}

	var requestID sql.NullString
	if rid := event.RequestID(); rid != "" {
		requestID = sql.NullString{String: rid, Valid: true}
	}

	var clientIP sql.NullString
	if cip := event.ClientIP(); cip != "" {
		clientIP = sql.NullString{String: cip, Valid: true}
	}

	_, err := r.db.ExecContext(ctx, query,
		event.ID(),
		event.OccurredAt(),
		userID,
		string(event.Kind()),
		string(event.Source()),
		reason,
		requestID,
		clientIP,
	)
	if err != nil {
		span.RecordError(err)
		r.o11y.Logger().Error(ctx, "identity.repository.auth_events.insert_failed",
			observability.String("layer", "repository"),
			observability.String("operation", "insert"),
			observability.Error(err),
		)
		return fmt.Errorf("%s insert: %w", prefixAuthEventsRepository, err)
	}
	return nil
}

func (r *authEventsRepository) AnonymizeByUserID(ctx context.Context, userID uuid.UUID) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "identity.repository.auth_events.anonymize_by_user_id")
	defer span.End()

	const query = `
		UPDATE auth_events
		   SET user_id = NULL
		 WHERE user_id = $1
	`

	_, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		span.RecordError(err)
		r.o11y.Logger().Error(ctx, "identity.repository.auth_events.anonymize_failed",
			observability.String("layer", "repository"),
			observability.String("operation", "anonymize_by_user_id"),
			observability.Error(err),
		)
		return fmt.Errorf("%s anonymize_by_user_id: %w", prefixAuthEventsRepository, err)
	}
	return nil
}

func (r *authEventsRepository) DeleteOlderThan(ctx context.Context, cutoff time.Time, batchSize int) (int64, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "identity.repository.auth_events.delete_older_than")
	defer span.End()

	const query = `
		DELETE FROM auth_events
		 WHERE id IN (
			SELECT id
			  FROM auth_events
			 WHERE occurred_at < $1
			 LIMIT $2
		)
	`

	result, err := r.db.ExecContext(ctx, query, cutoff, batchSize)
	if err != nil {
		span.RecordError(err)
		r.o11y.Logger().Error(ctx, "identity.repository.auth_events.delete_older_than_failed",
			observability.String("layer", "repository"),
			observability.String("operation", "delete_older_than"),
			observability.Error(err),
		)
		return 0, fmt.Errorf("%s delete_older_than: %w", prefixAuthEventsRepository, err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("%s delete_older_than rows_affected: %w", prefixAuthEventsRepository, err)
	}
	return n, nil
}
