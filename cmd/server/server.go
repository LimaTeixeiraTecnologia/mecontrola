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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions"
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

//nolint:revive // composition root agrega bootstrap de o11y, db, modules e shutdown; refatorar em helpers menores fragmentaria a ordem de lifecycle critica (HTTP -> Dispatcher -> Limiter -> Consumer -> Housekeeping -> PG).
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
		OTLPEndpoint:    cfg.O11yConfig.NormalizedExporterEndpoint(),
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
		manager.WithStartupMigrationDir(".migrations-disabled"),
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

	serverOpts := []httpserver.Option{
		httpserver.WithServiceName(cfg.HTTPConfig.ServiceNameAPI),
		httpserver.WithServiceVersion(cfg.O11yConfig.ServiceVersion),
		httpserver.WithEnvironment(cfg.AppConfig.Environment),
		httpserver.WithPort(strconv.Itoa(cfg.HTTPConfig.Port)),
		httpserver.WithMetrics(),
		httpserver.WithTracing(),
		httpserver.WithOTelMetrics(),
		httpserver.WithHealthChecks(map[string]httpserver.HealthCheckFunc{
			"database": dbManager.Ping,
		}),
		httpserver.WithShutdownTimeout(15 * time.Second),
	}
	if origins := resolveCORSOrigins(cfg); origins != "" {
		serverOpts = append(serverOpts, httpserver.WithCORS(origins))
	}

	srv, err := httpserver.New(
		o11y,
		serverOpts...,
	)
	if err != nil {
		return fmt.Errorf("run: failed to create http server: %w", err)
	}

	o11y.Logger().Info(ctx, "http server bootstrap completed",
		observability.String("service", cfg.HTTPConfig.ServiceNameAPI),
		observability.String("safe_dsn", cfg.DBConfig.SafeDSN()),
	)

	identityModule, err := identity.NewIdentityModule(cfg, o11y, dbManager)
	if err != nil {
		return fmt.Errorf("run: inicializar modulo identity: %w", err)
	}
	if identityModule.UserRouter != nil {
		srv.RegisterRouters(identityModule.UserRouter)
	}

	limiterStartCtx, limiterStartCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer limiterStartCancel()
	if err := identityModule.WhatsAppLimiter.Start(limiterStartCtx); err != nil {
		return fmt.Errorf("run: iniciar whatsapp limiter: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := identityModule.WhatsAppLimiter.Shutdown(shutdownCtx); err != nil {
			slog.Error("whatsapp limiter shutdown failed", "error", err)
		}
	}()

	o11y.Logger().Info(ctx, "identity module wired", observability.Bool("router_registered", identityModule.UserRouter != nil))

	categoriesModule := categories.NewCategoriesModule(dbManager, o11y)
	if categoriesModule.CategoryRouter != nil {
		srv.RegisterRouters(categoriesModule.CategoryRouter)
	}
	o11y.Logger().Info(ctx, "categories module wired", observability.Bool("router_registered", categoriesModule.CategoryRouter != nil))

	billingModule, err := billing.NewBillingModule(cfg, o11y, dbManager)
	if err != nil {
		return fmt.Errorf("run: inicializar modulo billing: %w", err)
	}
	if billingModule.WebhookRouter != nil {
		srv.RegisterRouters(billingModule.WebhookRouter)
	}
	o11y.Logger().Info(ctx, "billing module wired", observability.Bool("router_registered", billingModule.WebhookRouter != nil))

	onboardingModule, err := onboarding.NewOnboardingModule(
		dbManager,
		cfg.OnboardingConfig,
		cfg.WhatsAppConfig,
		cfg.TelegramConfig,
		cfg.OutboxConfig,
		identityModule,
		o11y,
	)
	if err != nil {
		return fmt.Errorf("run: inicializar modulo onboarding: %w", err)
	}
	srv.RegisterRouters(onboardingModule.PublicRouter)
	o11y.Logger().Info(ctx, "onboarding module wired")

	cardModule, err := card.NewCardModule(ctx, cfg, o11y, dbManager, identityModule.GatewayAuthMiddleware)
	if err != nil {
		return fmt.Errorf("run: inicializar modulo card: %w", err)
	}
	if cardModule.CardRouter != nil {
		srv.RegisterRouters(cardModule.CardRouter)
	}
	o11y.Logger().Info(ctx, "card module wired", observability.Bool("router_registered", cardModule.CardRouter != nil))

	budgetsModule, err := budgets.NewBudgetsModule(cfg, o11y, dbManager, categoriesModule)
	if err != nil {
		return fmt.Errorf("run: inicializar modulo budgets: %w", err)
	}
	if budgetsModule.BudgetsRouter != nil {
		srv.RegisterRouters(budgetsModule.BudgetsRouter)
	}
	o11y.Logger().Info(ctx, "budgets module wired", observability.Bool("router_registered", budgetsModule.BudgetsRouter != nil))

	transactionsModule, err := transactions.NewTransactionsModule(cfg, o11y, dbManager, cardModule, categoriesModule)
	if err != nil {
		return fmt.Errorf("run: inicializar modulo transactions: %w", err)
	}
	if transactionsModule.Router != nil {
		srv.RegisterRouters(transactionsModule.Router)
	}
	o11y.Logger().Info(ctx, "transactions module wired", observability.Bool("router_registered", transactionsModule.Router != nil))

	agentModule, err := agent.NewAgentModule(
		cfg,
		o11y,
		identityModule,
		categoriesModule,
		cardModule,
		transactionsModule,
		budgetsModule,
		onboardingModule.WhatsAppGateway,
	)
	if err != nil {
		return fmt.Errorf("run: inicializar modulo agent: %w", err)
	}
	o11y.Logger().Info(ctx, "agent module wired", observability.String("mode", agentModule.Mode))

	waWebhookRouter := composeWhatsAppWebhookRouter(cfg, o11y, identityModule, onboardingModule, agentModule)
	srv.RegisterRouters(waWebhookRouter)
	o11y.Logger().Info(ctx, "whatsapp webhook router wired", observability.String("path", "/api/v1/whatsapp"))

	if cfg.TelegramConfig.Enabled {
		telegramOnboardingRoute := buildTelegramOnboardingRoute(o11y, cfg.TelegramConfig, onboardingModule.TelegramMessageProcessor)
		tgRouter, tgErr := identityModule.BuildTelegramWebhookRouter(agentModule.TelegramAgentRoute, telegramOnboardingRoute)
		if tgErr != nil {
			return fmt.Errorf("run: compor telegram webhook router: %w", tgErr)
		}
		if tgRouter != nil {
			srv.RegisterRouters(tgRouter)
			o11y.Logger().Info(ctx, "telegram webhook router wired",
				observability.String("path", cfg.TelegramConfig.WebhookPath),
				observability.Int64("bot_id", cfg.TelegramConfig.BotID),
			)
		}
	} else {
		o11y.Logger().Info(ctx, "telegram webhook router skipped (TELEGRAM_ENABLED=false)")
	}

	if err := srv.Start(ctx); err != nil {
		return fmt.Errorf("run: http server stopped with error: %w", err)
	}
	return nil
}

func resolveCORSOrigins(cfg *configs.Config) string {
	return cfg.HTTPConfig.CORSAllowedOrigins
}
