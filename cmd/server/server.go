package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/database/postgres"
	httpserver "github.com/JailtonJunior94/devkit-go/pkg/http_server/chi_server"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/otel"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity"
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

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := dbManager.Shutdown(shutdownCtx); err != nil {
			slog.Error("database manager shutdown failed", "error", err)
		}
	}()

	srv, err := httpserver.New(
		o11y,
		httpserver.WithServiceName(cfg.HTTPConfig.ServiceNameAPI),
		httpserver.WithServiceVersion(cfg.O11yConfig.ServiceVersion),
		httpserver.WithEnvironment(cfg.AppConfig.Environment),
		httpserver.WithPort(strconv.Itoa(cfg.HTTPConfig.Port)),
		httpserver.WithCORS(resolveCORSOrigins(cfg)),
		httpserver.WithMetrics(),
		httpserver.WithTracing(),
		httpserver.WithOTelMetrics(),
		httpserver.WithHealthChecks(map[string]httpserver.HealthCheckFunc{
			"database": dbManager.Ping,
		}),
		httpserver.WithShutdownTimeout(15*time.Second),
	)
	if err != nil {
		return fmt.Errorf("run: failed to create http server: %w", err)
	}

	o11y.Logger().Info(ctx, "http server bootstrap completed",
		observability.String("service", cfg.HTTPConfig.ServiceNameAPI),
		observability.String("safe_dsn", cfg.DBConfig.SafeDSN()),
	)

	identityModule := identity.NewIdentityModule(cfg, o11y, dbManager)
	if identityModule.UserRouter != nil {
		srv.RegisterRouters(identityModule.UserRouter)
	}
	o11y.Logger().Info(ctx, "identity module wired", observability.Bool("router_registered", identityModule.UserRouter != nil))

	if err := srv.Start(ctx); err != nil {
		return fmt.Errorf("run: http server stopped with error: %w", err)
	}
	return nil
}

func resolveCORSOrigins(cfg *configs.Config) string {
	if origins := cfg.HTTPConfig.CORSAllowedOrigins; origins != "" {
		return origins
	}
	return "*"
}
