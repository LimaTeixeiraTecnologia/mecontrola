package database

import (
	"context"
	"fmt"

	devkitmanager "github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/database/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
)

// Manager wraps devkit-go's manager.Manager adding application-level helpers.
// It is created once in the composition root and injected into all modules.
// Reference: ADR-002 (single shared pool, D-08).
type Manager struct {
	inner devkitmanager.Manager
	// dsn stores the pgx5-scheme DSN used by RunMigrations (golang-migrate requires it).
	// It is set at construction time and never logged (R-SEC-001).
	dsn string
}

// NewManager creates a new Manager from the application configuration.
// It establishes the pgx pool, performs the initial ping and, if migrations
// are embedded via RunMigrations, they should be called after this.
// Pool size: up to 30 connections as documented in the discovery (D-08).
func NewManager(cfg *configs.Config) (*Manager, error) {
	pgCfg := postgres.PostgresConfig{
		Host:         cfg.DBConfig.Host,
		Port:         cfg.DBConfig.Port,
		User:         cfg.DBConfig.User,
		Password:     cfg.DBConfig.Password,
		Database:     cfg.DBConfig.Name,
		SSLMode:      cfg.DBConfig.SSLMode,
		MaxOpenConns: 30,
		MaxIdleConns: cfg.DBConfig.MinConns,
	}

	inner, err := devkitmanager.New(pgCfg)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConnection, err)
	}

	// Build the pgx5:// DSN used by golang-migrate.
	dsn := buildMigrationDSN(pgCfg)

	return &Manager{inner: inner, dsn: dsn}, nil
}

// buildMigrationDSN constructs the pgx5:// DSN required by golang-migrate/pgx/v5.
func buildMigrationDSN(cfg postgres.PostgresConfig) string {
	if cfg.DSN != "" {
		return normalizeToPgx5(cfg.DSN)
	}

	port := cfg.Port
	if port == 0 {
		port = postgres.DefaultPort
	}

	sslMode := cfg.SSLMode
	if sslMode == "" {
		sslMode = postgres.DefaultSSLMode
	}

	return fmt.Sprintf(
		"pgx5://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.User, cfg.Password, cfg.Host, port, cfg.Database, sslMode,
	)
}

// normalizeToPgx5 replaces postgres:// or postgresql:// prefix with pgx5://.
func normalizeToPgx5(dsn string) string {
	switch {
	case len(dsn) > 11 && dsn[:11] == "postgres://":
		return "pgx5" + dsn[8:]
	case len(dsn) > 13 && dsn[:13] == "postgresql://":
		return "pgx5" + dsn[10:]
	default:
		return dsn
	}
}

// Inner returns the underlying devkit-go manager for use by the UoW factory
// and any component that needs direct manager access.
func (m *Manager) Inner() devkitmanager.Manager {
	return m.inner
}

// HealthCheck verifies the database connection by performing a SELECT from
// the health_probe table. Returns ErrConnection if the check fails.
func (m *Manager) HealthCheck(ctx context.Context) error {
	dbtx := m.inner.DBTX(ctx)
	row := dbtx.QueryRowContext(ctx, "SELECT note FROM health_probe LIMIT 1")

	var note string
	if err := row.Scan(&note); err != nil {
		return fmt.Errorf("%w: health probe query failed: %w", ErrConnection, err)
	}

	return nil
}

// Shutdown gracefully closes the underlying connection pool.
func (m *Manager) Shutdown(ctx context.Context) error {
	return m.inner.Shutdown(ctx)
}
