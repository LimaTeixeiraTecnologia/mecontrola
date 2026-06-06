package migrate

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/database/migration"
	"github.com/JailtonJunior94/devkit-go/pkg/database/postgres"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/otel"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
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

	migrator, err := migration.New(
		rt.dbManager,
		migration.EmbedFS{FS: migrations.FS, Root: "."},
		migration.WithDSN(rt.cfg.DBConfig.DSN()),
		migration.WithMigrationTimeout(5*time.Minute),
		migration.WithObservability(rt.o11y),
	)
	if err != nil {
		return fmt.Errorf("migrate: erro ao criar migrator: %w", err)
	}

	if err := migrator.Up(ctx); err != nil {
		if errors.Is(err, migration.ErrNoChange) {
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

	migrator, err := migration.New(
		rt.dbManager,
		migration.EmbedFS{FS: migrations.FS, Root: "."},
		migration.WithDSN(rt.cfg.DBConfig.DSN()),
		migration.WithMigrationTimeout(5*time.Minute),
		migration.WithObservability(rt.o11y),
	)
	if err != nil {
		return fmt.Errorf("migrate-down: erro ao criar migrator: %w", err)
	}

	if err := migrator.Down(ctx, steps); err != nil {
		if errors.Is(err, migration.ErrNoChange) {
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
	dbManager manager.Manager
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
		OTLPEndpoint:    cfg.O11yConfig.ExporterEndpoint,
		Insecure:        cfg.O11yConfig.ExporterInsecure,
		LogLevel:        observability.LogLevel(cfg.O11yConfig.LogLevel),
		OTLPProtocol:    otel.OTLPProtocol(cfg.O11yConfig.ExporterProtocol),
		LogFormat:       observability.LogFormat(cfg.O11yConfig.LogFormat),
	}
	o11y, err := otel.NewProvider(ctx, o11yConfig)
	if err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	postgresConfig := postgres.PostgresConfig{
		DSN:          cfg.DBConfig.DSN(),
		MaxOpenConns: cfg.DBConfig.MaxConns,
		MaxIdleConns: cfg.DBConfig.MaxIdleConns,
		ConnMaxLife:  cfg.DBConfig.ConnMaxLifetime,
		ConnMaxIdle:  cfg.DBConfig.ConnMaxIdleTime,
	}
	dbManager, err := manager.New(
		postgresConfig,
		manager.WithObservability(o11y),
		manager.WithShutdownTimeout(10*time.Second),
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
