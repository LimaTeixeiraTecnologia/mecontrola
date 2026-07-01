package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/jmoiron/sqlx"
)

type AdvisoryKeyLocker struct {
	db   *sqlx.DB
	o11y observability.Observability
}

func NewAdvisoryKeyLocker(db *sqlx.DB, o11y observability.Observability) *AdvisoryKeyLocker {
	return &AdvisoryKeyLocker{db: db, o11y: o11y}
}

func (l *AdvisoryKeyLocker) WithKeyLock(ctx context.Context, key string, fn func(context.Context) error) error {
	ctx, span := l.o11y.Tracer().Start(ctx, "agents.persistence.advisory_key_locker.with_key_lock")
	defer span.End()

	conn, err := l.db.Connx(ctx)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("agents.persistence.advisory_key_locker: obter conexão: %w", err)
	}
	defer func() { _ = conn.Close() }()

	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock(hashtext($1))", key); err != nil {
		span.RecordError(err)
		return fmt.Errorf("agents.persistence.advisory_key_locker: adquirir lock: %w", err)
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = conn.ExecContext(unlockCtx, "SELECT pg_advisory_unlock(hashtext($1))", key)
	}()

	return fn(ctx)
}
