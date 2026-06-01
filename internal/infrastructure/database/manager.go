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

// managerBuilder constrói o DSN de migração a partir de postgres.PostgresConfig.
// Separado de Manager para que NewManager inicialize Manager em uma única expressão.
type managerBuilder struct {
	pgCfg postgres.PostgresConfig
}

// buildDSN retorna o DSN no esquema pgx5 para uso com golang-migrate.
func (b *managerBuilder) buildDSN() string {
	if b.pgCfg.DSN != "" {
		return b.normalizeToPgx5(b.pgCfg.DSN)
	}

	port := b.pgCfg.Port
	if port == 0 {
		port = postgres.DefaultPort
	}

	sslMode := b.pgCfg.SSLMode
	if sslMode == "" {
		sslMode = postgres.DefaultSSLMode
	}

	return fmt.Sprintf(
		"pgx5://%s:%s@%s:%d/%s?sslmode=%s",
		b.pgCfg.User, b.pgCfg.Password, b.pgCfg.Host, port, b.pgCfg.Database, sslMode,
	)
}

// normalizeToPgx5 converte DSNs com esquema postgres:// ou postgresql:// para pgx5://.
func (b *managerBuilder) normalizeToPgx5(dsn string) string {
	switch {
	case len(dsn) > 11 && dsn[:11] == "postgres://":
		return "pgx5" + dsn[8:]
	case len(dsn) > 13 && dsn[:13] == "postgresql://":
		return "pgx5" + dsn[10:]
	default:
		return dsn
	}
}

func NewManager(cfg *configs.Config) (*Manager, error) {
	maxConns := cfg.DBConfig.MaxConns
	if maxConns == 0 {
		maxConns = 30
	}

	pgCfg := postgres.PostgresConfig{
		Host:         cfg.DBConfig.Host,
		Port:         cfg.DBConfig.Port,
		User:         cfg.DBConfig.User,
		Password:     cfg.DBConfig.Password,
		Database:     cfg.DBConfig.Name,
		SSLMode:      cfg.DBConfig.SSLMode,
		MaxOpenConns: maxConns,
		MaxIdleConns: cfg.DBConfig.MinConns,
	}

	inner, err := devkitmanager.New(pgCfg)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConnection, err)
	}

	return &Manager{
		inner: inner,
		dsn:   (&managerBuilder{pgCfg: pgCfg}).buildDSN(),
	}, nil
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
