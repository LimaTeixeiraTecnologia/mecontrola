package database

import (
	"context"
	"errors"
	"fmt"

	migratelib "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/LimaTeixeiraTecnologia/mecontrola/migrations"
)

// RunMigrations applies all pending up-migrations using the embedded SQL files.
// It is idempotent: golang-migrate ErrNoChange is treated as success.
// Reference: ADR-007, RF-03, RF-12.
//
// Usage (from cmd/migrate/cmd.go):
//
//	if err := database.RunMigrations(ctx, mgr); err != nil { ... }
func RunMigrations(ctx context.Context, m *Manager) error {
	_ = ctx // reserved for future cancellation support

	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("%w: creating iofs source: %w", ErrMigration, err)
	}

	migrator, err := migratelib.NewWithSourceInstance("iofs", src, m.dsn)
	if err != nil {
		return fmt.Errorf("%w: creating migrator: %w", ErrMigration, err)
	}
	defer func() {
		_, _ = migrator.Close()
	}()

	if err := migrator.Up(); err != nil && !errors.Is(err, migratelib.ErrNoChange) {
		return fmt.Errorf("%w: applying migrations: %w", ErrMigration, err)
	}

	return nil
}

// RunMigrationsDown reverts all applied migrations (used in tests and runbook).
func RunMigrationsDown(ctx context.Context, m *Manager) error {
	_ = ctx

	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("%w: creating iofs source: %w", ErrMigration, err)
	}

	migrator, err := migratelib.NewWithSourceInstance("iofs", src, m.dsn)
	if err != nil {
		return fmt.Errorf("%w: creating migrator: %w", ErrMigration, err)
	}
	defer func() {
		_, _ = migrator.Close()
	}()

	if err := migrator.Down(); err != nil && !errors.Is(err, migratelib.ErrNoChange) {
		return fmt.Errorf("%w: reverting migrations: %w", ErrMigration, err)
	}

	return nil
}
