package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/otel"
	"github.com/jmoiron/sqlx"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

	"net/http"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent"
	agentbinding "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/binding"
	agentonboarding "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/onboarding"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/bootstrap"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/dedup"
	deduphandlers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/dedup/jobs/handlers"
	deduppostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/dedup/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/worker"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions"
)

func New() *cobra.Command {
	return &cobra.Command{
		Use:   "worker",
		Short: "Sobe o worker MeControla",
		Long:  "Inicializa o worker do MeControla com módulos de processamento em background.",
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
		ServiceName:     cfg.HTTPConfig.ServiceNameWorker,
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
		return fmt.Errorf("worker: failed to create observability provider: %w", err)
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
			fmt.Errorf("worker: erro ao inicializar database manager: %w", err),
			o11y.Shutdown(shutdownCtx),
		)
	}

	runtime := workerRuntime{cfg: cfg, o11y: o11y, dbManager: dbManager, db: sqlx.NewDb(dbManager.DB(), "pgx")}
	workerManager, err := runtime.newManager(ctx)
	if err != nil {
		return err
	}
	if err := workerManager.Start(ctx); err != nil {
		dbStartCtx, dbStartCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer dbStartCancel()
		o11yStartCtx, o11yStartCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer o11yStartCancel()
		return errors.Join(
			fmt.Errorf("worker: erro ao iniciar worker manager: %w", err),
			dbManager.Shutdown(dbStartCtx),
			o11y.Shutdown(o11yStartCtx),
		)
	}

	o11y.Logger().Info(
		ctx,
		"database manager initialized",
		observability.String("service", cfg.HTTPConfig.ServiceNameWorker),
		observability.String("safe_dsn", cfg.DBConfig.SafeDSN()),
	)
	o11y.Logger().Info(
		ctx,
		"worker bootstrap completed",
		observability.String("service", cfg.HTTPConfig.ServiceNameWorker),
	)
	<-ctx.Done()
	o11y.Logger().Info(
		context.Background(),
		"shutdown signal received, draining",
	)

	return runtime.shutdown(workerManager)
}

type workerRuntime struct {
	cfg       *configs.Config
	o11y      observability.Observability
	dbManager *postgres.Database
	db        *sqlx.DB
}

func (r *workerRuntime) newManager(ctx context.Context) (*worker.Manager, error) { //nolint:revive // composition root agrega bootstrap de módulos; refatorar fragmentaria a ordem de lifecycle crítica
	outboxFactory := outbox.NewRepositoryFactory(r.o11y)
	dispatcherUoW := uow.NewUnitOfWork(r.db)
	reaperUoW := uow.NewUnitOfWork(r.db)
	housekeepUoW := uow.NewUnitOfWork(r.db)
	eventsDispatcher := events.NewDispatcher()
	identityModule, err := identity.NewIdentityModule(r.cfg, r.o11y, r.db)
	if err != nil {
		return nil, fmt.Errorf("worker: inicializar modulo identity: %w", err)
	}
	passthroughGateway := func(next http.Handler) http.Handler { return next }
	categoriesModule := categories.NewCategoriesModule(r.db, r.o11y, passthroughGateway)
	billingModule, err := billing.NewBillingModule(r.cfg, r.o11y, r.db)
	if err != nil {
		return nil, fmt.Errorf("worker: inicializar modulo billing: %w", err)
	}
	onboardingModule, err := onboarding.NewOnboardingModule(
		r.db,
		r.cfg.OnboardingConfig,
		r.cfg.WhatsAppConfig,
		r.cfg.TelegramConfig,
		r.cfg.OutboxConfig,
		r.cfg.EmailConfig,
		identityModule,
		r.o11y,
	)
	if err != nil {
		return nil, fmt.Errorf("worker: inicializar modulo onboarding: %w", err)
	}

	channelGateway, err := bootstrap.BuildChannelGateway(r.cfg, r.o11y, onboardingModule.WhatsAppGateway)
	if err != nil {
		return nil, fmt.Errorf("worker: construir channel gateway: %w", err)
	}
	channelResolver := bootstrap.BuildBudgetsChannelResolver(identityModule)
	cardChannelResolver := bootstrap.BuildCardChannelResolver(identityModule)

	cardModule, err := card.NewCardModule(ctx, r.cfg, r.o11y, r.db, passthroughGateway, channelGateway, cardChannelResolver)
	if err != nil {
		return nil, fmt.Errorf("worker: inicializar modulo card: %w", err)
	}
	onboardingModule.SaveOnboardingCard.SetCardCreator(agentbinding.NewOnboardingCardCreatorAdapter(cardModule.CreateCardUC))
	transactionsModule, err := transactions.NewTransactionsModule(r.cfg, r.o11y, r.db, cardModule, categoriesModule, passthroughGateway)
	if err != nil {
		return nil, fmt.Errorf("worker: inicializar modulo transactions: %w", err)
	}

	budgetsModule, err := budgets.NewBudgetsModule(r.cfg, r.o11y, r.db, categoriesModule, passthroughGateway, channelGateway, channelResolver)
	if err != nil {
		return nil, fmt.Errorf("worker: inicializar modulo budgets: %w", err)
	}

	agentModule, err := agent.NewAgentModule(
		r.cfg,
		r.o11y,
		identityModule,
		categoriesModule,
		cardModule,
		transactionsModule,
		budgetsModule,
		onboardingModule.WhatsAppGateway,
		agentonboarding.NewBudgetConfiguratorAdapter(onboardingModule.StartBudgetConfiguration),
		agentonboarding.NewOnboardingContinuationAdapter(onboardingModule.WhatsAppMessageProcessor, onboardingModule.TelegramMessageProcessor),
		agent.WithSessionStore(r.db),
		agent.WithOutboxPublisher(identityModule.OutboxPublisher),
	)
	if err != nil {
		return nil, fmt.Errorf("worker: inicializar modulo agent: %w", err)
	}

	for _, reg := range identityModule.EventHandlers {
		if err := eventsDispatcher.Register(reg.EventType, reg.Handler); err != nil {
			return nil, fmt.Errorf("worker: registrar handler identity %s: %w", reg.EventType, err)
		}
	}
	for _, reg := range agentModule.EventHandlers {
		if err := eventsDispatcher.Register(reg.EventType, reg.Handler); err != nil {
			return nil, fmt.Errorf("worker: registrar handler agent %s: %w", reg.EventType, err)
		}
	}
	for _, reg := range billingModule.EventHandlers {
		if err := eventsDispatcher.Register(reg.EventType, reg.Handler); err != nil {
			return nil, fmt.Errorf("worker: registrar handler billing %s: %w", reg.EventType, err)
		}
	}
	for _, reg := range onboardingModule.EventHandlers {
		if err := eventsDispatcher.Register(reg.EventType, reg.Handler); err != nil {
			return nil, fmt.Errorf("worker: registrar handler onboarding %s: %w", reg.EventType, err)
		}
	}
	for _, reg := range budgetsModule.EventHandlers {
		if err := eventsDispatcher.Register(reg.EventType, reg.Handler); err != nil {
			return nil, fmt.Errorf("worker: registrar handler budgets %s: %w", reg.EventType, err)
		}
	}
	for _, reg := range transactionsModule.EventHandlers {
		if err := eventsDispatcher.Register(reg.EventType, reg.Handler); err != nil {
			return nil, fmt.Errorf("worker: registrar handler transactions %s: %w", reg.EventType, err)
		}
	}
	for _, reg := range cardModule.EventHandlers {
		if err := eventsDispatcher.Register(reg.EventType, reg.Handler); err != nil {
			return nil, fmt.Errorf("worker: registrar handler card %s: %w", reg.EventType, err)
		}
	}

	dedupRepo := deduppostgres.NewMessageRepository(r.o11y, r.db)
	dedupCleanup := dedup.NewCleanupProcessedMessages(dedupRepo, r.cfg.WhatsAppConfig, r.o11y)
	dedupHousekeepingJob := deduphandlers.NewDedupHousekeepingJob(dedupCleanup, r.cfg.WhatsAppConfig)

	jobs := make([]worker.Job, 0, 10)
	if r.cfg.OutboxConfig.DispatcherEnabled {
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		jobs = append(jobs, outbox.NewObservableDispatcherJob(dispatcherUoW, outboxFactory, eventsDispatcher, r.cfg.OutboxConfig, r.o11y, rng))
	}
	jobs = append(jobs,
		outbox.NewReaperJob(reaperUoW, outboxFactory, r.cfg.OutboxConfig, r.o11y.Logger()),
		outbox.NewHousekeepingJob(housekeepUoW, outboxFactory, r.cfg.OutboxConfig, r.o11y.Logger()),
		identityModule.AuthEventsHousekeepingJob,
		dedupHousekeepingJob,
		billingModule.ReconciliationJob,
		billingModule.KiwifyEventsHousekeeper,
		billingModule.GraceExpirationJob,
		onboardingModule.OutreachJob,
		onboardingModule.ExpirationJob,
		onboardingModule.MetaProcessedMessagesCleanup,
		budgetsModule.AbandonedDraftReaper,
		budgetsModule.PendingEventsReaper,
		budgetsModule.RetentionPurge,
	)
	if budgetsModule.ThresholdAlertsJob != nil {
		jobs = append(jobs, budgetsModule.ThresholdAlertsJob)
	}
	if cardModule.InvoiceDueAlertsJob != nil {
		jobs = append(jobs, cardModule.InvoiceDueAlertsJob)
	}
	if transactionsModule.RecurringMaterializerJob != nil {
		jobs = append(jobs, transactionsModule.RecurringMaterializerJob)
	}
	if transactionsModule.MonthlySummaryReconcilerJob != nil {
		jobs = append(jobs, transactionsModule.MonthlySummaryReconcilerJob)
	}

	schedLogger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	return worker.NewManager(worker.Config{ShutdownTimeout: 30 * time.Second}, jobs, nil, schedLogger), nil
}

func (r *workerRuntime) shutdown(workerManager *worker.Manager) error {
	var shutdownErrs []error

	workerCtx, workerCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer workerCancel()
	if err := workerManager.Stop(workerCtx); err != nil {
		shutdownErrs = append(shutdownErrs, fmt.Errorf("worker: erro ao parar worker manager: %w", err))
	}

	dbCtx, dbCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer dbCancel()
	if err := r.dbManager.Shutdown(dbCtx); err != nil {
		shutdownErrs = append(shutdownErrs, fmt.Errorf("worker: erro ao encerrar database manager: %w", err))
	}

	o11yCtx, o11yCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer o11yCancel()
	if err := r.o11y.Shutdown(o11yCtx); err != nil {
		shutdownErrs = append(shutdownErrs, fmt.Errorf("worker: erro durante shutdown de observabilidade: %w", err))
	}

	return errors.Join(shutdownErrs...)
}
