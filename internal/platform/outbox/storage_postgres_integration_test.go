//go:build integration

package outbox_test

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/database/migration"
	dbpostgres "github.com/JailtonJunior94/devkit-go/pkg/database/postgres"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/migrations"
)

const pgImage = "postgres:16"

func setupOutboxDB(t *testing.T) manager.Manager {
	t.Helper()
	ctx := context.Background()

	req := tc.ContainerRequest{
		Image:        pgImage,
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "testdb",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(60 * time.Second),
	}

	container, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err, "start postgres container")

	t.Cleanup(func() {
		if terr := container.Terminate(context.Background()); terr != nil {
			t.Logf("container terminate: %v", terr)
		}
	})

	host, err := container.Host(ctx)
	require.NoError(t, err)

	mapped, err := container.MappedPort(ctx, "5432")
	require.NoError(t, err)

	portNum, err := strconv.Atoi(mapped.Port())
	require.NoError(t, err)

	cfg := dbpostgres.PostgresConfig{
		Host:     host,
		Port:     portNum,
		User:     "test",
		Password: "test",
		Database: "testdb",
		SSLMode:  "disable",
	}

	mgr, err := manager.New(cfg)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = mgr.Shutdown(context.Background())
	})

	dsn := fmt.Sprintf("pgx5://test:test@%s:%d/testdb?sslmode=disable", host, portNum)
	migrator, err := migration.New(mgr, migration.EmbedFS{FS: migrations.FS, Root: "."}, migration.WithDSN(dsn))
	require.NoError(t, err)

	if err := migrator.Up(ctx); err != nil && !errors.Is(err, migration.ErrNoChange) {
		t.Fatalf("run migrations: %v", err)
	}

	return mgr
}

func newEvent(t *testing.T, id string) outbox.Event {
	t.Helper()
	if id == "" {
		id = uuid.NewString()
	}
	evt, err := outbox.NewEvent(outbox.EventInput{
		ID:            id,
		Type:          "billing.subscription.activated",
		AggregateType: "subscription",
		AggregateID:   uuid.NewString(),
		Payload:       []byte(`{"foo":"bar"}`),
		Metadata:      map[string]string{"source": "test"},
		OccurredAt:    time.Now().UTC().Add(-time.Minute),
	})
	require.NoError(t, err)
	return evt
}

func countStatus(t *testing.T, mgr manager.Manager, id string) int {
	t.Helper()
	ctx := context.Background()
	var status int
	err := mgr.DBTX(ctx).QueryRowContext(ctx,
		`SELECT status FROM outbox_events WHERE id = $1`, id,
	).Scan(&status)
	require.NoError(t, err)
	return status
}

func countRows(t *testing.T, mgr manager.Manager, id string) int {
	t.Helper()
	ctx := context.Background()
	var n int
	err := mgr.DBTX(ctx).QueryRowContext(ctx,
		`SELECT COUNT(*) FROM outbox_events WHERE id = $1`, id,
	).Scan(&n)
	require.NoError(t, err)
	return n
}

func TestOutboxStorage_InsertIsIdempotentByID(t *testing.T) {
	mgr := setupOutboxDB(t)
	ctx := context.Background()
	storage := outbox.NewPostgresStorage(mgr.DBTX(ctx))

	evt := newEvent(t, "")

	require.NoError(t, storage.Insert(ctx, evt, 10))
	require.NoError(t, storage.Insert(ctx, evt, 10), "second insert with same id must not error")
	require.NoError(t, storage.Insert(ctx, evt, 10), "third insert with same id must not error")

	require.Equal(t, 1, countRows(t, mgr, evt.ID),
		"ON CONFLICT (id) DO NOTHING must keep exactly one row")
	require.Equal(t, int(outbox.StatusPending), countStatus(t, mgr, evt.ID))
}

func TestOutboxStorage_MarkPublishedTransitionsState(t *testing.T) {
	mgr := setupOutboxDB(t)
	ctx := context.Background()
	storage := outbox.NewPostgresStorage(mgr.DBTX(ctx))

	evt := newEvent(t, "")
	require.NoError(t, storage.Insert(ctx, evt, 5))
	require.Equal(t, int(outbox.StatusPending), countStatus(t, mgr, evt.ID))

	require.NoError(t, storage.MarkPublished(ctx, evt.ID))
	require.Equal(t, int(outbox.StatusPublished), countStatus(t, mgr, evt.ID))
}

func TestOutboxStorage_MarkFailedTransitionsState(t *testing.T) {
	mgr := setupOutboxDB(t)
	ctx := context.Background()
	storage := outbox.NewPostgresStorage(mgr.DBTX(ctx))

	evt := newEvent(t, "")
	require.NoError(t, storage.Insert(ctx, evt, 3))

	require.NoError(t, storage.MarkFailed(ctx, evt.ID, "exhausted"))
	require.Equal(t, int(outbox.StatusFailed), countStatus(t, mgr, evt.ID))

	var lastErr string
	require.NoError(t, mgr.DBTX(ctx).QueryRowContext(ctx,
		`SELECT last_error FROM outbox_events WHERE id = $1`, evt.ID,
	).Scan(&lastErr))
	require.Equal(t, "exhausted", lastErr)
}

func TestOutboxStorage_MarkPendingRetryIncrementsAttempts(t *testing.T) {
	mgr := setupOutboxDB(t)
	ctx := context.Background()
	storage := outbox.NewPostgresStorage(mgr.DBTX(ctx))

	evt := newEvent(t, "")
	require.NoError(t, storage.Insert(ctx, evt, 5))

	next := time.Now().UTC().Add(30 * time.Second)
	require.NoError(t, storage.MarkPendingRetry(ctx, evt.ID, "transient", next))

	var attempts int
	var lastErr string
	require.NoError(t, mgr.DBTX(ctx).QueryRowContext(ctx,
		`SELECT attempts, last_error FROM outbox_events WHERE id = $1`, evt.ID,
	).Scan(&attempts, &lastErr))
	require.Equal(t, 1, attempts)
	require.Equal(t, "transient", lastErr)
	require.Equal(t, int(outbox.StatusPending), countStatus(t, mgr, evt.ID))
}

func TestOutboxStorage_ClaimBatchLocksAndReturnsRows(t *testing.T) {
	mgr := setupOutboxDB(t)
	ctx := context.Background()
	storage := outbox.NewPostgresStorage(mgr.DBTX(ctx))

	for i := 0; i < 3; i++ {
		require.NoError(t, storage.Insert(ctx, newEvent(t, ""), 5))
	}

	rows, err := storage.ClaimBatch(ctx, "worker-1", 10)
	require.NoError(t, err)
	require.Len(t, rows, 3)

	for _, r := range rows {
		require.Equal(t, int(outbox.StatusProcessing), countStatus(t, mgr, r.ID))
	}

	again, err := storage.ClaimBatch(ctx, "worker-2", 10)
	require.NoError(t, err)
	require.Empty(t, again, "claimed rows must not be re-claimed before reset")
}

func TestOutboxStorage_DeletePublishedBatchRespectsRetention(t *testing.T) {
	mgr := setupOutboxDB(t)
	ctx := context.Background()
	storage := outbox.NewPostgresStorage(mgr.DBTX(ctx))

	old := newEvent(t, "")
	require.NoError(t, storage.Insert(ctx, old, 5))
	require.NoError(t, storage.MarkPublished(ctx, old.ID))

	_, err := mgr.DBTX(ctx).ExecContext(ctx,
		`UPDATE outbox_events SET published_at = now() - interval '2 hours' WHERE id = $1`, old.ID,
	)
	require.NoError(t, err)

	recent := newEvent(t, "")
	require.NoError(t, storage.Insert(ctx, recent, 5))
	require.NoError(t, storage.MarkPublished(ctx, recent.ID))

	deleted, err := storage.DeletePublishedBatch(ctx, time.Hour, 100)
	require.NoError(t, err)
	require.Equal(t, int64(1), deleted, "must delete only rows older than retention")

	require.Equal(t, 0, countRows(t, mgr, old.ID), "old row must be gone")
	require.Equal(t, 1, countRows(t, mgr, recent.ID), "recent row must be preserved")
}

func TestOutboxStorage_ResetStuckReleasesProcessing(t *testing.T) {
	mgr := setupOutboxDB(t)
	ctx := context.Background()
	storage := outbox.NewPostgresStorage(mgr.DBTX(ctx))

	evt := newEvent(t, "")
	require.NoError(t, storage.Insert(ctx, evt, 5))

	claimed, err := storage.ClaimBatch(ctx, "worker-A", 10)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.Equal(t, int(outbox.StatusProcessing), countStatus(t, mgr, evt.ID))

	_, err = mgr.DBTX(ctx).ExecContext(ctx,
		`UPDATE outbox_events SET locked_at = now() - interval '10 minutes' WHERE id = $1`, evt.ID,
	)
	require.NoError(t, err)

	n, err := storage.ResetStuck(ctx, 5*time.Minute)
	require.NoError(t, err)
	require.Equal(t, int64(1), n)
	require.Equal(t, int(outbox.StatusPending), countStatus(t, mgr, evt.ID))
}
