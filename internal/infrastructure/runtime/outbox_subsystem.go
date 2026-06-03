package runtime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/observability"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/outbox"
)

// lazyOutboxSubsystem constrói as dependências (DB + OTel + Subsystem) no momento do Start,
// não no momento do Bootstrap. Isso permite que Bootstrap seja chamado em testes
// sem um banco real disponível.
//
// closers são fechados em ordem inversa no Stop, garantindo que a ordem de deinit
// seja o inverso da ordem de init (D-15 / techspec "Pontos de Integração").
type lazyOutboxSubsystem struct {
	cfg     *configs.Config
	found   Foundation
	sub     *outbox.Subsystem
	closers []func(context.Context) error
}

func (b *bootstrapper) newOutboxSubsystem(cfg *configs.Config, f Foundation) *lazyOutboxSubsystem {
	return &lazyOutboxSubsystem{cfg: cfg, found: f}
}

// Name retorna o identificador deste subsistema no runtime.
func (s *lazyOutboxSubsystem) Name() string { return "outbox" }

// Start constrói Manager → Provider → Registry → Storage → Metrics → Subsystem em sequência.
// Em caso de erro em qualquer etapa, fecha os recursos já abertos via closers em ordem inversa.
func (s *lazyOutboxSubsystem) Start(ctx context.Context) error {
	mgr, err := database.NewManager(s.cfg)
	if err != nil {
		return fmt.Errorf("outbox subsystem: database: %w", err)
	}
	s.closers = append(s.closers, mgr.Shutdown)

	prov, shutProv, err := observability.NewProvider(s.cfg)
	if err != nil {
		_ = s.closeAll(context.Background())
		return fmt.Errorf("outbox subsystem: observability: %w", err)
	}
	s.closers = append(s.closers, shutProv)

	registry := outbox.NewRegistry()
	if err := registerSubscriptions(registry); err != nil {
		_ = s.closeAll(context.Background())
		return fmt.Errorf("outbox subsystem: registry: %w", err)
	}

	metrics, err := outbox.NewOutboxMetrics(prov.Observability())
	if err != nil {
		_ = s.closeAll(context.Background())
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
		_ = s.closeAll(context.Background())
		return fmt.Errorf("outbox subsystem: build: %w", err)
	}
	s.sub = sub

	return sub.Start(ctx)
}

// Stop para o Subsystem e fecha todos os closers em ordem inversa à criação.
func (s *lazyOutboxSubsystem) Stop(ctx context.Context) error {
	var errs []error
	if s.sub != nil {
		if err := s.sub.Stop(ctx); err != nil {
			errs = append(errs, fmt.Errorf("outbox shutdown: %w", err))
		}
	}
	if err := s.closeAll(ctx); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// closeAll fecha os closers em ordem inversa à criação.
func (s *lazyOutboxSubsystem) closeAll(ctx context.Context) error {
	var errs []error
	for i := len(s.closers) - 1; i >= 0; i-- {
		if err := s.closers[i](ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
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
