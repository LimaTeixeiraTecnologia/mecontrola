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

	httpserver "github.com/JailtonJunior94/devkit-go/pkg/http_server/chi_server"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/otel"
	"github.com/jmoiron/sqlx"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/infrastructure/weather"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/bootstrap"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/http/server/health"
	openapidocs "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/http/server/openapi"
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

	db := sqlx.NewDb(dbManager.DB(), "pgx")

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
	if origins := cfg.HTTPConfig.CORSAllowedOrigins; origins != "" {
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

	identityModule, err := identity.NewIdentityModule(cfg, o11y, db)
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

	categoriesModule := categories.NewCategoriesModule(db, o11y, identityModule.GatewayAuthMiddleware)
	if categoriesModule.CategoryRouter != nil {
		srv.RegisterRouters(categoriesModule.CategoryRouter)
	}
	o11y.Logger().Info(ctx, "categories module wired", observability.Bool("router_registered", categoriesModule.CategoryRouter != nil))

	billingModule, err := billing.NewBillingModule(cfg, o11y, db)
	if err != nil {
		return fmt.Errorf("run: inicializar modulo billing: %w", err)
	}
	if billingModule.WebhookRouter != nil {
		srv.RegisterRouters(billingModule.WebhookRouter)
	}
	o11y.Logger().Info(ctx, "billing module wired", observability.Bool("router_registered", billingModule.WebhookRouter != nil))

	onboardingModule, err := onboarding.NewOnboardingModule(
		db,
		cfg.OnboardingConfig,
		cfg.WhatsAppConfig,
		cfg.OutboxConfig,
		cfg.EmailConfig,
		identityModule,
		o11y,
	)
	if err != nil {
		return fmt.Errorf("run: inicializar modulo onboarding: %w", err)
	}
	srv.RegisterRouters(onboardingModule.PublicRouter)
	o11y.Logger().Info(ctx, "onboarding module wired")

	cardModule, err := card.NewCardModule(ctx, cfg, o11y, db, identityModule.GatewayAuthMiddleware, nil, nil)
	if err != nil {
		return fmt.Errorf("run: inicializar modulo card: %w", err)
	}
	o11y.Logger().Info(ctx, "card module initialized", observability.Bool("router_present", cardModule.CardRouter != nil))
	channelGateway, err := bootstrap.BuildChannelGateway(cfg, o11y, onboardingModule.WhatsAppGateway)
	if err != nil {
		return fmt.Errorf("run: build channel gateway: %w", err)
	}
	channelResolver := bootstrap.BuildBudgetsChannelResolver(identityModule)
	budgetsModule, err := budgets.NewBudgetsModule(cfg, o11y, db, categoriesModule, identityModule.GatewayAuthMiddleware, channelGateway, channelResolver)
	if err != nil {
		return fmt.Errorf("run: inicializar modulo budgets: %w", err)
	}
	if budgetsModule.BudgetsRouter != nil {
		srv.RegisterRouters(budgetsModule.BudgetsRouter)
	}
	o11y.Logger().Info(ctx, "budgets module wired", observability.Bool("router_registered", budgetsModule.BudgetsRouter != nil))

	transactionsModule, err := transactions.NewTransactionsModule(cfg, o11y, db, cardModule, categoriesModule, identityModule.GatewayAuthMiddleware)
	if err != nil {
		return fmt.Errorf("run: inicializar modulo transactions: %w", err)
	}
	if cardModule.CardRouter != nil && transactionsModule.GetCardInvoiceHandler != nil {
		cardModule.CardRouter.WithInvoiceByMonthHandler(transactionsModule.GetCardInvoiceHandler)
	}
	if cardModule.CardRouter != nil {
		srv.RegisterRouters(cardModule.CardRouter)
	}
	o11y.Logger().Info(ctx, "card module wired", observability.Bool("router_registered", cardModule.CardRouter != nil))
	if transactionsModule.Router != nil {
		srv.RegisterRouters(transactionsModule.Router)
	}
	o11y.Logger().Info(ctx, "transactions module wired", observability.Bool("router_registered", transactionsModule.Router != nil))

	agentsModule, err := agents.NewModule(agents.Deps{
		DB:              db,
		O11y:            o11y,
		OutboxPublisher: identityModule.OutboxPublisher,
		LLM: agents.LLMConfig{
			Model:       cfg.AgentConfig.PrimaryModel,
			EmbedModel:  cfg.AgentConfig.EmbedModel,
			APIKey:      cfg.AgentConfig.OpenRouterAPIKey,
			BaseURL:     cfg.AgentConfig.OpenRouterBaseURL,
			MaxTokens:   cfg.AgentConfig.MaxTokens,
			Temperature: cfg.AgentConfig.Temperature,
		},
		WeatherClient:   weather.NewClient(),
		WhatsAppGateway: onboardingModule.WhatsAppGateway,
	})
	if err != nil {
		return fmt.Errorf("run: inicializar modulo agents: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		agentsModule.Shutdown(shutdownCtx)
	}()
	o11y.Logger().Info(ctx, "agents module wired")

	waWebhookRouter := composeWhatsAppWebhookRouter(cfg, o11y, identityModule, agentsModule, onboardingModule)
	srv.RegisterRouters(waWebhookRouter)
	o11y.Logger().Info(ctx, "whatsapp webhook router wired", observability.String("path", "/api/v1/whatsapp"))

	srv.RegisterRouters(health.NewReadinessRouter(ctx))

	docsRouter, err := openapidocs.NewRouterIfEnabled(cfg)
	if err != nil {
		return fmt.Errorf("run: inicializar docs openapi locais: %w", err)
	}
	if docsRouter != nil {
		srv.RegisterRouters(docsRouter)
		o11y.Logger().Info(ctx, "local openapi docs wired", observability.String("path", "/__docs"))
	}

	if err := srv.Start(ctx); err != nil {
		return fmt.Errorf("run: http server stopped with error: %w", err)
	}
	return nil
}
