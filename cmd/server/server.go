package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/database/postgres"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/otel"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
)

func New() *cobra.Command {
	return &cobra.Command{
		Use:   "server",
		Short: "Sobe o servidor HTTP MeControla",
		Long:  "Inicializa o servidor HTTP do MeControla com composição por módulos.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Run()
		},
	}
}

func Run() error {
	cfg, err := configs.LoadConfig(".")
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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
	o11y, err := otel.NewProvider(context.Background(), o11yConfig)
	if err != nil {
		return fmt.Errorf("run: failed to create observability provider: %w", err)
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
		manager.WithPoolStatsInterval(30*time.Second),
	)
	if err != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return errors.Join(
			fmt.Errorf("run: erro ao inicializar database manager: %w", err),
			o11y.Shutdown(shutdownCtx),
		)
	}

	o11y.Logger().Info(ctx, "database manager initialized",
		observability.String("service", cfg.HTTPConfig.ServiceNameAPI),
		observability.String("safe_dsn", cfg.DBConfig.SafeDSN()),
	)
	o11y.Logger().Info(ctx, "server bootstrap completed", observability.String("service", cfg.HTTPConfig.ServiceNameAPI))
	<-ctx.Done()
	o11y.Logger().Info(context.Background(), "shutdown signal received, draining")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var shutdownErrs []error
	if err := dbManager.Shutdown(shutdownCtx); err != nil {
		shutdownErrs = append(shutdownErrs, fmt.Errorf("run: erro ao encerrar database manager: %w", err))
	}
	if err := o11y.Shutdown(shutdownCtx); err != nil {
		shutdownErrs = append(shutdownErrs, fmt.Errorf("run: erro durante shutdown de observabilidade: %w", err))
	}
	return errors.Join(shutdownErrs...)
}
