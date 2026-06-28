package migrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/otel"
	"github.com/golang-migrate/migrate/v4"
	migratepgx "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/migrations"
)

func New() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Aplica migrations pendentes do banco de dados",
		Long:  "Executa todas as migrations pendentes via golang-migrate e termina com exit code 0 em sucesso.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Run(cmd.OutOrStdout())
		},
	}
}

func NewDown() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate-down",
		Short: "Reverte migrations aplicadas",
		Long:  "Reverte migrations via golang-migrate. Default: 1 step. Use --steps N (ou -1 para todas).",
	}
	steps := cmd.Flags().Int("steps", 1, "número de migrations a reverter (-1 para todas)")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return RunDown(cmd.OutOrStdout(), *steps)
	}
	return cmd
}

const migrationAdvisoryLockID int64 = 424242

func acquireMigrationLock(ctx context.Context, db *sql.DB) (func(), error) {
	var acquired bool
	err := db.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", migrationAdvisoryLockID).Scan(&acquired)
	if err != nil {
		return nil, fmt.Errorf("advisory lock query: %w", err)
	}
	if !acquired {
		return nil, fmt.Errorf("outro processo de migrate esta em execucao")
	}
	return func() {
		_, _ = db.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", migrationAdvisoryLockID)
	}, nil
}

func Run(writer io.Writer) (retErr error) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	rt, err := bootstrap(ctx)
	if err != nil {
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	defer func() {
		retErr = errors.Join(retErr, rt.shutdown(shutdownCtx))
	}()

	unlock, err := acquireMigrationLock(ctx, rt.dbManager.DB())
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	defer unlock()

	migrator, err := rt.newMigrator()
	if err != nil {
		return fmt.Errorf("migrate: erro ao criar migrator: %w", err)
	}

	if err := migrator.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			rt.o11y.Logger().Info(ctx, "no pending migrations",
				observability.String("service", rt.cfg.HTTPConfig.ServiceNameAPI),
				observability.String("op", "migrate"),
			)
			return nil
		}
		return fmt.Errorf("migrate: %w", err)
	}

	rt.o11y.Logger().Info(ctx, "migrations applied",
		observability.String("service", rt.cfg.HTTPConfig.ServiceNameAPI),
		observability.String("op", "migrate"),
	)
	_, err = fmt.Fprintln(writer, "migrations applied")
	return err
}

func RunDown(writer io.Writer, steps int) (retErr error) {
	if steps == 0 {
		return fmt.Errorf("migrate-down: steps deve ser != 0")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	rt, err := bootstrap(ctx)
	if err != nil {
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	defer func() {
		retErr = errors.Join(retErr, rt.shutdown(shutdownCtx))
	}()

	unlock, err := acquireMigrationLock(ctx, rt.dbManager.DB())
	if err != nil {
		return fmt.Errorf("migrate-down: %w", err)
	}
	defer unlock()

	migrator, err := rt.newMigrator()
	if err != nil {
		return fmt.Errorf("migrate-down: erro ao criar migrator: %w", err)
	}

	if err := migrateDown(migrator, steps); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			rt.o11y.Logger().Info(ctx, "no migrations to revert",
				observability.String("service", rt.cfg.HTTPConfig.ServiceNameAPI),
				observability.String("op", "migrate-down"),
			)
			return nil
		}
		return fmt.Errorf("migrate-down: %w", err)
	}

	rt.o11y.Logger().Info(ctx, "migrations reverted",
		observability.String("service", rt.cfg.HTTPConfig.ServiceNameAPI),
		observability.String("op", "migrate-down"),
		observability.Int("steps", steps),
	)
	_, err = fmt.Fprintf(writer, "migrations reverted (steps=%d)\n", steps)
	return err
}

type runtime struct {
	cfg       *configs.Config
	o11y      observability.Observability
	dbManager *postgres.Database
}

func (r *runtime) newMigrator() (*migrate.Migrate, error) {
	if _, err := r.dbManager.DB().ExecContext(context.Background(), `CREATE SCHEMA IF NOT EXISTS mecontrola`); err != nil {
		return nil, fmt.Errorf("migrate: garantir schema mecontrola: %w", err)
	}
	driver, err := migratepgx.WithInstance(r.dbManager.DB(), &migratepgx.Config{
		MigrationsTable: migratepgx.DefaultMigrationsTable,
		SchemaName:      "mecontrola",
	})
	if err != nil {
		return nil, fmt.Errorf("migrate: criar driver: %w", err)
	}
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("migrate: criar source: %w", err)
	}
	return migrate.NewWithInstance("iofs", src, "pgx5", driver)
}

func migrateDown(migrator *migrate.Migrate, steps int) error {
	if steps < 0 {
		return migrator.Down()
	}
	return migrator.Steps(-steps)
}

func (r *runtime) shutdown(ctx context.Context) error {
	dbErr := r.dbManager.Shutdown(ctx)
	if err := r.o11y.Shutdown(ctx); err != nil {
		slog.Warn("migrate: observability shutdown failed", "error", err)
	}
	return dbErr
}

func bootstrap(ctx context.Context) (*runtime, error) {
	cfg, err := configs.LoadConfig(".")
	if err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	o11yConfig := &otel.Config{
		Environment:     cfg.AppConfig.Environment,
		ServiceName:     cfg.HTTPConfig.ServiceNameAPI,
		ServiceVersion:  cfg.O11yConfig.ServiceVersion,
		TraceSampleRate: cfg.O11yConfig.TraceSampleRate,
		OTLPEndpoint:    cfg.O11yConfig.NormalizedExporterEndpoint(),
		Insecure:        cfg.O11yConfig.ExporterInsecure,
		LogLevel:        observability.LogLevel(cfg.O11yConfig.LogLevel),
		OTLPProtocol:    otel.OTLPProtocol(cfg.O11yConfig.ExporterProtocol),
		LogFormat:       observability.LogFormat(cfg.O11yConfig.LogFormat),
	}
	o11y, err := otel.NewProvider(ctx, o11yConfig)
	if err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	dbManager, err := postgres.New(
		cfg.DBConfig.DSN(),
		postgres.WithMaxOpenConns(cfg.DBConfig.MaxConns),
		postgres.WithMaxIdleConns(cfg.DBConfig.MaxIdleConns),
		postgres.WithConnMaxLifetime(cfg.DBConfig.ConnMaxLifetime),
		postgres.WithConnMaxIdleTime(cfg.DBConfig.ConnMaxIdleTime),
	)
	if err != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return nil, errors.Join(
			fmt.Errorf("migrate: erro ao inicializar database manager: %w", err),
			o11y.Shutdown(shutdownCtx),
		)
	}

	return &runtime{cfg: cfg, o11y: o11y, dbManager: dbManager}, nil
}
