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

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/database/postgres"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/otel"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/worker"
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
		OTLPEndpoint:    cfg.O11yConfig.ExporterEndpoint,
		Insecure:        cfg.O11yConfig.ExporterInsecure,
		LogLevel:        observability.LogLevel(cfg.O11yConfig.LogLevel),
		OTLPProtocol:    otel.OTLPProtocol(cfg.O11yConfig.ExporterProtocol),
		LogFormat:       observability.LogFormat(cfg.O11yConfig.LogFormat),
	}
	o11y, err := otel.NewProvider(context.Background(), o11yConfig)
	if err != nil {
		return fmt.Errorf("worker: failed to create observability provider: %w", err)
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
			fmt.Errorf("worker: erro ao inicializar database manager: %w", err),
			o11y.Shutdown(shutdownCtx),
		)
	}

	runtime := workerRuntime{cfg: cfg, o11y: o11y, dbManager: dbManager}
	workerManager, err := runtime.newManager()
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

	o11y.Logger().Info(ctx, "database manager initialized",
		observability.String("service", cfg.HTTPConfig.ServiceNameWorker),
		observability.String("safe_dsn", cfg.DBConfig.SafeDSN()),
	)
	o11y.Logger().Info(ctx, "worker bootstrap completed", observability.String("service", cfg.HTTPConfig.ServiceNameWorker))
	<-ctx.Done()
	o11y.Logger().Info(context.Background(), "shutdown signal received, draining")

	return runtime.shutdown(workerManager)
}

type workerRuntime struct {
	cfg       *configs.Config
	o11y      observability.Observability
	dbManager manager.Manager
}

func (r *workerRuntime) newManager() (*worker.Manager, error) {
	outboxFactory := outbox.NewRepositoryFactory(r.o11y)
	dispatcherUoW := uow.New[[]outbox.Row](r.dbManager, uow.WithObservability(r.o11y))
	reaperUoW := uow.NewVoid(r.dbManager, uow.WithObservability(r.o11y))
	housekeepUoW := uow.NewVoid(r.dbManager, uow.WithObservability(r.o11y))
	eventsDispatcher := events.NewDispatcher()
	identityModule := identity.NewIdentityModule(r.cfg, r.o11y, r.dbManager)
	billingModule, err := billing.NewBillingModule(r.cfg, r.o11y, r.dbManager)
	if err != nil {
		return nil, fmt.Errorf("worker: inicializar modulo billing: %w", err)
	}

	onboardingModule, err := onboarding.NewOnboardingModule(
		r.dbManager,
		r.cfg.OnboardingConfig,
		r.cfg.WhatsAppConfig,
		r.cfg.OutboxConfig,
		identityModule,
		r.o11y,
	)
	if err != nil {
		return nil, fmt.Errorf("worker: inicializar modulo onboarding: %w", err)
	}

	for _, reg := range identityModule.EventHandlers {
		if err := eventsDispatcher.Register(reg.EventType, reg.Handler); err != nil {
			return nil, fmt.Errorf("worker: registrar handler identity %s: %w", reg.EventType, err)
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

	jobs := make([]worker.Job, 0, 10)
	if r.cfg.OutboxConfig.DispatcherEnabled {
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		jobs = append(jobs, outbox.NewDispatcherJob(dispatcherUoW, outboxFactory, eventsDispatcher, r.cfg.OutboxConfig, r.o11y.Logger(), rng))
	}
	jobs = append(jobs,
		outbox.NewReaperJob(reaperUoW, outboxFactory, r.cfg.OutboxConfig, r.o11y.Logger()),
		outbox.NewHousekeepingJob(housekeepUoW, outboxFactory, r.cfg.OutboxConfig, r.o11y.Logger()),
		identityModule.AuthEventsHousekeepingJob,
		billingModule.ReconciliationJob,
		billingModule.KiwifyEventsHousekeeper,
		billingModule.GraceExpirationJob,
		onboardingModule.OutreachJob,
		onboardingModule.ExpirationJob,
		onboardingModule.MetaProcessedMessagesCleanup,
	)

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
