package outbox

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
)

// SubsystemDeps agrupa todas as dependências externas de Subsystem (DI via construtor, R6.6).
type SubsystemDeps struct {
	// Config contém a configuração completa do outbox.
	Config configs.OutboxConfig
	// Storage é a porta de acesso ao banco de dados.
	Storage Storage
	// Registry é o registro de subscriptions populado no bootstrap.
	Registry Registry
	// Metrics é a fachada OTel para instrumentação.
	Metrics *OutboxMetrics
	// Logger é o logger estruturado. Se nil, usa slog.Default().
	Logger *slog.Logger
	// Clock é a fonte de tempo injetável. Se nil, usa RealClock().
	Clock Clock
	// InstanceID identifica esta instância do worker para coordenação multi-instância (D-11).
	InstanceID string
}

// Subsystem é o agregador que implementa runtime.Subsystem compondo Dispatcher e Cron.
// Implementa Start/Stop/Name conforme o contrato runtime.Subsystem (RF-09 / RF-39).
//
// Ciclo de vida:
//   - Start: chama cron.Start sempre; chama dispatcher.Start somente se enabled.
//   - Stop: cancela dispatcher e cron em paralelo; agrega erros via errors.Join.
//
// Object Calisthenics #8: 5 campos principais (dispatcher, cron, logger, instanceID, enabled).
type Subsystem struct {
	dispatcher *Dispatcher
	cron       *Cron
	logger     *slog.Logger
	instanceID string
	enabled    bool
}

// NewSubsystem cria e valida um Subsystem a partir de SubsystemDeps.
// Constrói Dispatcher e Cron internamente. Chama registry.Validate() antes de retornar.
func NewSubsystem(deps SubsystemDeps) (*Subsystem, error) {
	if deps.Storage == nil {
		return nil, fmt.Errorf("outbox: subsystem: storage é obrigatório")
	}
	if deps.Registry == nil {
		return nil, fmt.Errorf("outbox: subsystem: registry é obrigatório")
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	if deps.Clock == nil {
		deps.Clock = RealClock()
	}
	if deps.InstanceID == "" {
		deps.InstanceID = NewInstanceID()
	}

	if err := deps.Registry.Validate(); err != nil {
		return nil, fmt.Errorf("outbox: subsystem: registry.Validate: %w", err)
	}

	policy, err := NewBackoffPolicy(
		max(deps.Config.RetryBaseBackoff, time.Second),
		max(deps.Config.RetryMaxBackoff, 5*time.Minute),
		rand.New(rand.NewSource(time.Now().UnixNano())), //nolint:gosec // seed nao precisa ser criptografico
	)
	if err != nil {
		return nil, fmt.Errorf("outbox: subsystem: backoff policy: %w", err)
	}

	dispatcher, err := NewDispatcher(DispatcherConfig{
		Enabled:        deps.Config.DispatcherEnabled,
		Storage:        deps.Storage,
		Registry:       deps.Registry,
		Policy:         policy,
		MaxAttempts:    NewAttempt(uint8(deps.Config.RetryMaxAttempts)), //nolint:gosec // valor validado em [1..50]
		HandlerTimeout: deps.Config.DispatcherHandlerTimeout,
		TickInterval:   deps.Config.DispatcherTickInterval,
		BatchSize:      deps.Config.DispatcherBatchSize,
		InstanceID:     deps.InstanceID,
		Clock:          deps.Clock,
		Metrics:        newMetricsAdapter(deps.Metrics),
		Logger:         deps.Logger,
	})
	if err != nil {
		return nil, fmt.Errorf("outbox: subsystem: dispatcher: %w", err)
	}

	retentionDays := deps.Config.HousekeepingRetentionDays
	if retentionDays <= 0 {
		retentionDays = 90
	}
	reaperStuckAfter := deps.Config.ReaperStuckAfter
	if reaperStuckAfter <= 0 {
		reaperStuckAfter = 5 * time.Minute
	}

	cronDeps := CronDeps{
		Storage:              deps.Storage,
		Metrics:              deps.Metrics,
		Logger:               deps.Logger,
		Clock:                deps.Clock,
		HousekeepingSchedule: deps.Config.HousekeepingSchedule,
		ReaperInterval:       deps.Config.ReaperInterval,
		RetentionDays:        retentionDays,
		ReaperStuckAfter:     reaperStuckAfter,
	}

	cron, err := NewCron(cronDeps)
	if err != nil {
		return nil, fmt.Errorf("outbox: subsystem: cron: %w", err)
	}

	return &Subsystem{
		dispatcher: dispatcher,
		cron:       cron,
		logger:     deps.Logger,
		instanceID: deps.InstanceID,
		enabled:    deps.Config.DispatcherEnabled,
	}, nil
}

// Name retorna o identificador deste subsistema no runtime (RF-09).
func (s *Subsystem) Name() string { return "outbox" }

// Start inicia o Cron sempre e o Dispatcher somente se enabled (RF-39).
// Loga "outbox.subsystem.started" com campos dispatcher_enabled e instance_id.
// Payload jamais aparece em chamada slog.* (R-SEC-001).
func (s *Subsystem) Start(ctx context.Context) error {
	if err := s.cron.Start(ctx); err != nil {
		return fmt.Errorf("outbox: subsystem: cron.Start: %w", err)
	}

	if err := s.dispatcher.Start(ctx); err != nil {
		return fmt.Errorf("outbox: subsystem: dispatcher.Start: %w", err)
	}

	s.logger.InfoContext(ctx, "outbox.subsystem.started",
		slog.Bool("dispatcher_enabled", s.enabled),
		slog.String("instance_id", s.instanceID),
	)
	return nil
}

// Stop cancela Dispatcher e Cron em paralelo, aguarda drenagem e agrega erros via errors.Join (RF-39).
// Respeita ctx para não bloquear indefinidamente em shutdown.
func (s *Subsystem) Stop(ctx context.Context) error {
	dispErrCh := make(chan error, 1)
	cronErrCh := make(chan error, 1)

	go func() { dispErrCh <- s.dispatcher.Stop(ctx) }()
	go func() { cronErrCh <- s.cron.Stop(ctx) }()

	dispErr := <-dispErrCh
	cronErr := <-cronErrCh

	return errors.Join(dispErr, cronErr)
}
