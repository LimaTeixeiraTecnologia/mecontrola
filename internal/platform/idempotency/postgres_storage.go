package idempotency

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

const pgUniqueViolation = "23505"

type postgresStorage struct {
	db database.DBTX
}

func NewPostgresStorage(db database.DBTX) Storage {
	return &postgresStorage{db: db}
}

func (s *postgresStorage) conn(ctx context.Context) database.DBTX {
	if tx, ok := database.FromContext(ctx); ok {
		return tx
	}
	return s.db
}

func (s *postgresStorage) Get(ctx context.Context, scope, key, userID string) (Record, error) {
	const q = `
		SELECT scope, key, user_id, request_hash, response_status, response_body, expires_at, created_at
		  FROM mecontrola.idempotency_keys
		 WHERE scope    = $1
		   AND key      = $2
		   AND user_id  = $3
		   AND expires_at > now()`

	row := s.conn(ctx).QueryRowContext(ctx, q, scope, key, userID)

	var rec Record
	err := row.Scan(
		&rec.Scope,
		&rec.Key,
		&rec.UserID,
		&rec.RequestHash,
		&rec.ResponseStatus,
		&rec.ResponseBody,
		&rec.ExpiresAt,
		&rec.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Record{}, ErrNotFound
		}
		return Record{}, fmt.Errorf("idempotency: get: %w", err)
	}
	return rec, nil
}

func (s *postgresStorage) Put(ctx context.Context, rec Record) error {
	const q = `
		INSERT INTO mecontrola.idempotency_keys
			(scope, key, user_id, request_hash, response_status, response_body, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (scope, key, user_id) DO NOTHING
		RETURNING scope`

	row := s.conn(ctx).QueryRowContext(ctx,
		q,
		rec.Scope,
		rec.Key,
		rec.UserID,
		rec.RequestHash,
		rec.ResponseStatus,
		rec.ResponseBody,
		rec.ExpiresAt,
	)

	var returnedScope string
	err := row.Scan(&returnedScope)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			existing, getErr := s.Get(ctx, rec.Scope, rec.Key, rec.UserID)
			if getErr != nil {
				return fmt.Errorf("idempotency: put conflict check: %w", getErr)
			}
			if existing.RequestHash != rec.RequestHash {
				return ErrHashMismatch
			}
			return nil
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			existing, getErr := s.Get(ctx, rec.Scope, rec.Key, rec.UserID)
			if getErr != nil {
				return fmt.Errorf("idempotency: put conflict check: %w", getErr)
			}
			if existing.RequestHash != rec.RequestHash {
				return ErrHashMismatch
			}
			return nil
		}
		return fmt.Errorf("idempotency: put: %w", err)
	}
	return nil
}
