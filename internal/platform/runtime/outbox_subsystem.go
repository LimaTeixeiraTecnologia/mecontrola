package runtime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/observability"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

// lazyOutboxSubsystem constrói as dependências (DB + OTel + Subsystem) no momento do Start,
// não no momento do Bootstrap. Isso permite que Bootstrap seja chamado em testes
// sem um banco real disponível.
type lazyOutboxSubsystem struct {
	cfg   *configs.Config
	found Foundation
	stop  func(context.Context) error
}

func (b *bootstrapper) newOutboxSubsystem(cfg *configs.Config, f Foundation) *lazyOutboxSubsystem {
	return &lazyOutboxSubsystem{cfg: cfg, found: f}
}

// Name retorna o identificador deste subsistema no runtime.
func (s *lazyOutboxSubsystem) Name() string { return "outbox" }

// Start constrói Manager → Provider → Registry → Storage → Metrics → Subsystem em sequência.
// Em caso de erro em qualquer etapa, fecha os recursos já abertos via closers locais em ordem inversa.
func (s *lazyOutboxSubsystem) Start(ctx context.Context) error {
	var closers []func(context.Context) error

	closeAll := func() {
		for i := len(closers) - 1; i >= 0; i-- {
			_ = closers[i](context.Background())
		}
	}

	mgr, err := database.NewManager(s.cfg)
	if err != nil {
		return fmt.Errorf("outbox subsystem: database: %w", err)
	}
	closers = append(closers, mgr.Shutdown)

	prov, shutProv, err := observability.NewProvider(s.cfg)
	if err != nil {
		closeAll()
		return fmt.Errorf("outbox subsystem: observability: %w", err)
	}
	closers = append(closers, shutProv)

	registry := outbox.NewRegistry()
	if err := registerSubscriptions(registry); err != nil {
		closeAll()
		return fmt.Errorf("outbox subsystem: registry: %w", err)
	}

	metrics, err := outbox.NewOutboxMetrics(prov.Observability())
	if err != nil {
		closeAll()
		return fmt.Errorf("outbox subsystem: metrics: %w", err)
	}

	sub, err := outbox.NewSubsystem(outbox.SubsystemDeps{
		Config:     s.cfg.OutboxConfig,
		Storage:    outbox.NewPgxStorage(mgr.Inner()),
		Registry:   registry,
		Metrics:    metrics,
		Logger:     slog.Default(),
		Clock:      s.found.Clock,
		InstanceID: outbox.NewInstanceID(),
	})
	if err != nil {
		closeAll()
		return fmt.Errorf("outbox subsystem: build: %w", err)
	}

	s.stop = buildOutboxStopFn(sub, closers)

	return sub.Start(ctx)
}

// Stop para o Subsystem e fecha todos os recursos em ordem inversa à criação.
func (s *lazyOutboxSubsystem) Stop(ctx context.Context) error {
	if s.stop == nil {
		return nil
	}
	return s.stop(ctx)
}

type outboxStopper interface {
	Stop(ctx context.Context) error
}

// buildOutboxStopFn cria o closure de shutdown capturando sub e closers em ordem inversa.
// closers são fechados após o sub para garantir que o subsistema pare antes dos recursos.
func buildOutboxStopFn(sub outboxStopper, closers []func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		var errs []error
		if err := sub.Stop(ctx); err != nil {
			errs = append(errs, fmt.Errorf("outbox shutdown: %w", err))
		}
		for i := len(closers) - 1; i >= 0; i-- {
			if err := closers[i](ctx); err != nil {
				errs = append(errs, err)
			}
		}
		return errors.Join(errs...)
	}
}

// registerSubscriptions registra as subscriptions no registry do worker.
// Cada subscription associa um event_type a um Handler idempotente.
// Payload jamais aparece em chamada slog.* (R-SEC-001).
func registerSubscriptions(registry outbox.Registry) error {
	dummyName, err := outbox.NewSubscriptionName("outbox-dummy")
	if err != nil {
		return fmt.Errorf("registerSubscriptions: %w", err)
	}

	dummyEventType, err := events.NewEventName("platform.outbox-dummy")
	if err != nil {
		return fmt.Errorf("registerSubscriptions: %w", err)
	}

	if err := registry.Register(outbox.Subscription{
		Name:      dummyName,
		EventType: dummyEventType,
		Handler:   outbox.DummyHandler,
	}); err != nil {
		return fmt.Errorf("registerSubscriptions: %w", err)
	}

	return nil
}
