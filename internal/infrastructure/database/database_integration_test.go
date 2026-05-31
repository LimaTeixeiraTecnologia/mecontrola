//go:build integration

package database_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	devkitdb "github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	dbpkg "github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/database"
)

// startPostgres starts an ephemeral postgres:16-alpine container and returns
// a *configs.Config pointing to it. The container is automatically stopped
// when the test ends.
func startPostgres(t *testing.T) *configs.Config {
	t.Helper()

	ctx := context.Background()

	container, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("testdb"),
		tcpostgres.WithUsername("testuser"),
		tcpostgres.WithPassword("testpassword"),
		tcpostgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = container.Terminate(context.Background())
	})

	host, err := container.Host(ctx)
	require.NoError(t, err)

	mappedPort, err := container.MappedPort(ctx, "5432")
	require.NoError(t, err)

	// Port.Num() returns the uint16 port number.
	port := int(mappedPort.Num())

	return &configs.Config{
		DBConfig: configs.DBConfig{
			Host:     host,
			Port:     port,
			User:     "testuser",
			Password: "testpassword",
			Name:     "testdb",
			SSLMode:  "disable",
			MaxConns: 5,
			MinConns: 1,
		},
	}
}

// TestIntegration_PoolStartup verifies that NewManager successfully connects to
// the Postgres container and that Ping succeeds.
func TestIntegration_PoolStartup(t *testing.T) {
	t.Parallel()

	cfg := startPostgres(t)
	mgr, err := dbpkg.NewManager(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = mgr.Shutdown(context.Background()) })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = mgr.Inner().Ping(ctx)
	assert.NoError(t, err)
}

// TestIntegration_MigrateUpDown verifies that RunMigrations and RunMigrationsDown
// apply and revert the 0001_init migration without error, and that ErrNoChange
// is tolerated on idempotent calls.
func TestIntegration_MigrateUpDown(t *testing.T) {
	t.Parallel()

	cfg := startPostgres(t)
	mgr, err := dbpkg.NewManager(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = mgr.Shutdown(context.Background()) })

	ctx := context.Background()

	// First up: must create health_probe.
	err = dbpkg.RunMigrations(ctx, mgr)
	require.NoError(t, err, "RunMigrations up should succeed")

	// Idempotent up: ErrNoChange must be swallowed.
	err = dbpkg.RunMigrations(ctx, mgr)
	require.NoError(t, err, "RunMigrations up (idempotent) should not error")

	// Down: must drop health_probe.
	err = dbpkg.RunMigrationsDown(ctx, mgr)
	require.NoError(t, err, "RunMigrationsDown should succeed")

	// Idempotent down: ErrNoChange must be swallowed.
	err = dbpkg.RunMigrationsDown(ctx, mgr)
	require.NoError(t, err, "RunMigrationsDown (idempotent) should not error")
}

// TestIntegration_HealthCheck verifies Manager.HealthCheck returns nil after
// migrations are applied and ErrConnection after migrations are reverted.
func TestIntegration_HealthCheck(t *testing.T) {
	t.Parallel()

	cfg := startPostgres(t)
	mgr, err := dbpkg.NewManager(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = mgr.Shutdown(context.Background()) })

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Apply migrations so health_probe table exists.
	require.NoError(t, dbpkg.RunMigrations(ctx, mgr))

	// HealthCheck must succeed.
	err = mgr.HealthCheck(ctx)
	assert.NoError(t, err, "HealthCheck should return nil when DB is healthy")

	// Revert migrations: health_probe table disappears.
	require.NoError(t, dbpkg.RunMigrationsDown(ctx, mgr))

	// HealthCheck must fail with ErrConnection wrapping.
	err = mgr.HealthCheck(ctx)
	require.Error(t, err)
	assert.True(t, errors.Is(err, dbpkg.ErrConnection),
		"expected ErrConnection after table removal, got: %v", err)
}

// TestIntegration_UoWCommit verifies that a UoW transaction is committed and
// the data is visible after the Do call.
func TestIntegration_UoWCommit(t *testing.T) {
	t.Parallel()

	cfg := startPostgres(t)
	mgr, err := dbpkg.NewManager(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = mgr.Shutdown(context.Background()) })

	ctx := context.Background()
	require.NoError(t, dbpkg.RunMigrations(ctx, mgr))

	uow := dbpkg.NewUnitOfWork[string](mgr)

	result, err := uow.Do(ctx, func(ctx context.Context, tx devkitdb.DBTX) (string, error) {
		row := tx.QueryRowContext(ctx, "SELECT note FROM health_probe LIMIT 1")
		var note string
		if err := row.Scan(&note); err != nil {
			return "", fmt.Errorf("scan: %w", err)
		}
		return note, nil
	})

	require.NoError(t, err)
	assert.Equal(t, "ok", result)
}

// TestIntegration_UoWRollback verifies that when a UoW fn returns an error,
// the transaction is rolled back. We insert a row and then return an error;
// the row must not be visible afterwards.
func TestIntegration_UoWRollback(t *testing.T) {
	t.Parallel()

	cfg := startPostgres(t)
	mgr, err := dbpkg.NewManager(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = mgr.Shutdown(context.Background()) })

	ctx := context.Background()
	require.NoError(t, dbpkg.RunMigrations(ctx, mgr))

	// Count rows before the attempted insert.
	rowsBefore := countHealthProbeRows(ctx, t, mgr)

	uow := dbpkg.NewUnitOfWork[struct{}](mgr)

	// Insert and then force rollback by returning an error.
	_, err = uow.Do(ctx, func(ctx context.Context, tx devkitdb.DBTX) (struct{}, error) {
		if _, execErr := tx.ExecContext(ctx, "INSERT INTO health_probe (note) VALUES ($1)", "rollback-test"); execErr != nil {
			return struct{}{}, fmt.Errorf("insert: %w", execErr)
		}
		// Return an error to trigger rollback.
		return struct{}{}, errors.New("intentional rollback")
	})
	require.Error(t, err)
	assert.EqualError(t, err, "intentional rollback")

	// Rows after rollback must equal rows before.
	rowsAfter := countHealthProbeRows(ctx, t, mgr)
	assert.Equal(t, rowsBefore, rowsAfter, "rollback must revert the INSERT")
}

// countHealthProbeRows is a test helper that counts rows in the health_probe table.
func countHealthProbeRows(ctx context.Context, t *testing.T, mgr *dbpkg.Manager) int {
	t.Helper()
	row := mgr.Inner().DBTX(ctx).QueryRowContext(ctx, "SELECT COUNT(*) FROM health_probe")
	var n int
	require.NoError(t, row.Scan(&n))
	return n
}
