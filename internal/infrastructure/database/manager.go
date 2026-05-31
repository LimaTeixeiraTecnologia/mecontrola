package database

import (
	"context"
	"fmt"

	devkitmanager "github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/database/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
)

type Manager struct {
	inner devkitmanager.Manager
	// dsn stores the pgx5-scheme DSN used by RunMigrations (golang-migrate requires it).
	// It is set at construction time and never logged (R-SEC-001).
	dsn string
}

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

	dsn := buildMigrationDSN(pgCfg)

	return &Manager{inner: inner, dsn: dsn}, nil
}

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

func (m *Manager) Inner() devkitmanager.Manager {
	return m.inner
}

func (m *Manager) HealthCheck(ctx context.Context) error {
	dbtx := m.inner.DBTX(ctx)
	row := dbtx.QueryRowContext(ctx, "SELECT note FROM health_probe LIMIT 1")

	var note string
	if err := row.Scan(&note); err != nil {
		return fmt.Errorf("%w: health probe query failed: %w", ErrConnection, err)
	}

	return nil
}

func (m *Manager) Shutdown(ctx context.Context) error {
	return m.inner.Shutdown(ctx)
}
