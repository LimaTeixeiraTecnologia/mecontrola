package outbox

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"go.opentelemetry.io/otel/propagation"
)

// Clock é a abstração de tempo injetável pelo Dispatcher.
// Permite testes determinísticos sem depender de time.Now().
type Clock interface {
	Now() time.Time
}

// realClock é a implementação de Clock usando time.Now().
type realClock struct{}

func (realClock) Now() time.Time { return time.Now().UTC() }

// RealClock retorna o Clock de produção baseado em time.Now().
func RealClock() Clock { return realClock{} }

// Metrics é a fachada de observabilidade injetada no Dispatcher.
// Permite mocking em testes e instrumentação OTel em produção.
type Metrics interface {
	// RecordDeliverySuccess registra uma entrega bem-sucedida.
	RecordDeliverySuccess(ctx context.Context, subscriptionName string)
	// RecordDeliveryFailure registra uma falha de entrega (transitória ou permanente).
	RecordDeliveryFailure(ctx context.Context, subscriptionName string)
	// RecordDeliveryDLQ registra uma entrega enviada para DLQ.
	RecordDeliveryDLQ(ctx context.Context, subscriptionName string)
}

// noopMetrics é uma implementação de Metrics que não faz nada.
type noopMetrics struct{}

func (noopMetrics) RecordDeliverySuccess(_ context.Context, _ string) {}
func (noopMetrics) RecordDeliveryFailure(_ context.Context, _ string) {}
func (noopMetrics) RecordDeliveryDLQ(_ context.Context, _ string)     {}

// NoopMetrics retorna uma implementação de Metrics que não faz nada.
// Útil para testes unitários que não validam métricas.
func NoopMetrics() Metrics { return noopMetrics{} }

type deliveryMetricsRecorder interface {
	recordDeliverySuccess(ctx context.Context, subscriptionName string, latencyMs float64)
	recordDeliveryFailure(ctx context.Context, subscriptionName string, err error)
	recordDeliveryDLQ(ctx context.Context, subscriptionName string)
}

type pollMetricsRecorder interface {
	recordPoll(ctx context.Context, duration time.Duration, batchSize int)
}

type pendingMetricsRecorder interface {
	setPendingDelta(ctx context.Context, subscriptionName string, delta int64)
}

type tracerMetricsProvider interface {
	tracer() observability.Tracer
}

// DispatcherConfig agrupa os parâmetros de construção do Dispatcher.
type DispatcherConfig struct {
	// Enabled controla se o loop de polling será iniciado.
	// Quando false, Start retorna imediatamente sem criar goroutine ou ticker.
	Enabled bool
	// Storage é a porta de acesso ao banco de dados.
	Storage Storage
	// Registry mapeia event_type para Subscriptions.
	Registry Registry
	// Policy define o backoff exponencial com jitter.
	Policy BackoffPolicy
	// MaxAttempts é o limite de tentativas antes de enviar para DLQ.
	MaxAttempts Attempt
	// HandlerTimeout é o timeout máximo para execução de um Handler.
	HandlerTimeout time.Duration
	// TickInterval é o intervalo entre ticks do Dispatcher.
	TickInterval time.Duration
	// BatchSize é o número máximo de Claims por tick.
	BatchSize int
	// InstanceID identifica esta instância do Dispatcher para coordenação multi-instância.
	InstanceID string
	// Clock é a fonte de tempo injetável. Se nil, usa RealClock().
	Clock Clock
	// Metrics é a fachada de observabilidade. Se nil, usa NoopMetrics().
	Metrics Metrics
	// Logger é o logger estruturado. Se nil, usa slog.Default().
	Logger *slog.Logger
}

// Dispatcher é o motor do Outbox: acorda no time.Ticker, faz claim de um lote
// via Storage.ClaimReady, executa cada Handler com timeout e classifica o resultado.
//
// Thread-safety: Dispatcher é seguro para uso a partir de uma única goroutine de controle
// (Start/Stop). Os handlers são executados em goroutines separadas controladas pelo WaitGroup.
//
// BackoffPolicy é protegido contra acesso concorrente ao RNG.
type Dispatcher struct {
	enabled        bool
	storage        Storage
	registry       Registry
	policy         BackoffPolicy
	maxAttempts    Attempt
	handlerTimeout time.Duration
	tickInterval   time.Duration
	batchSize      int
	instanceID     string
	clock          Clock
	metrics        Metrics
	tracer         observability.Tracer
	logger         *slog.Logger
	wg             sync.WaitGroup
	stopCh         chan struct{}
	mu             sync.Mutex
	pendingMu      sync.Mutex
	pendingCounts  map[string]int64
	lastStatsAt    time.Time
	// processedCounter é incrementado a cada entrega processada com sucesso.
	// O log INFO outbox.delivery.processed é emitido com sampler 1:100 (RF-36, subtarefa 9.3).
	processedCounter atomic.Uint64
}

// NewDispatcher cria um Dispatcher validado a partir de DispatcherConfig.
func NewDispatcher(cfg DispatcherConfig) (*Dispatcher, error) {
	if cfg.Storage == nil {
		return nil, fmt.Errorf("outbox: dispatcher: storage é obrigatório")
	}
	if cfg.Registry == nil {
		return nil, fmt.Errorf("outbox: dispatcher: registry é obrigatório")
	}
	if cfg.HandlerTimeout <= 0 {
		cfg.HandlerTimeout = 10 * time.Second
	}
	if cfg.TickInterval <= 0 {
		cfg.TickInterval = 500 * time.Millisecond
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 50
	}
	if cfg.MaxAttempts.Value() == 0 {
		cfg.MaxAttempts = NewAttempt(15)
	}
	if cfg.Clock == nil {
		cfg.Clock = RealClock()
	}
	if cfg.Metrics == nil {
		cfg.Metrics = NoopMetrics()
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	var tracer observability.Tracer
	if provider, ok := cfg.Metrics.(tracerMetricsProvider); ok {
		tracer = provider.tracer()
	}

	return &Dispatcher{
		enabled:        cfg.Enabled,
		storage:        cfg.Storage,
		registry:       cfg.Registry,
		policy:         cfg.Policy,
		maxAttempts:    cfg.MaxAttempts,
		handlerTimeout: cfg.HandlerTimeout,
		tickInterval:   cfg.TickInterval,
		batchSize:      cfg.BatchSize,
		instanceID:     cfg.InstanceID,
		clock:          cfg.Clock,
		metrics:        cfg.Metrics,
		tracer:         tracer,
		logger:         cfg.Logger,
		stopCh:         make(chan struct{}),
		pendingCounts:  make(map[string]int64),
	}, nil
}

// Start inicia o loop de polling do Dispatcher numa goroutine dedicada.
// Se OUTBOX_DISPATCHER_ENABLED=false, retorna imediatamente sem criar goroutine ou ticker.
// Chamar Start mais de uma vez tem comportamento indefinido.
func (d *Dispatcher) Start(ctx context.Context) error {
	if !d.enabled {
		d.logger.InfoContext(ctx, "outbox.dispatcher.disabled",
			slog.String("reason", "OUTBOX_DISPATCHER_ENABLED=false"),
		)
		return nil
	}

	d.logger.InfoContext(ctx, "outbox.dispatcher.started",
		slog.String("instance_id", d.instanceID),
		slog.Duration("tick_interval", d.tickInterval),
		slog.Int("batch_size", d.batchSize),
	)

	go d.loop(ctx)
	return nil
}

// Stop sinaliza o Dispatcher para parar e aguarda todos os handlers in-flight
// terminarem ou o ctx expirar (o que ocorrer primeiro).
func (d *Dispatcher) Stop(ctx context.Context) error {
	d.mu.Lock()
	select {
	case <-d.stopCh:
		// já fechado
	default:
		close(d.stopCh)
	}
	d.mu.Unlock()

	done := make(chan struct{})
	go func() {
		d.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		d.logger.InfoContext(ctx, "outbox: dispatcher parado — todos os handlers drenados")
		return nil
	case <-ctx.Done():
		d.logger.WarnContext(ctx, "outbox: dispatcher stop timeout — handlers in-flight podem ter sido abandonados")
		return ctx.Err()
	}
}

// loop é o laço principal do Dispatcher.
func (d *Dispatcher) loop(ctx context.Context) {
	ticker := time.NewTicker(d.tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.tickOnce(ctx)
		}
	}
}

// tickOnce executa um único tick: faz claim de um lote e despacha cada claim.
func (d *Dispatcher) tickOnce(ctx context.Context) {
	startedAt := d.clock.Now()
	claims, err := d.storage.ClaimReady(ctx, d.batchSize, d.instanceID)
	if err != nil {
		d.logger.ErrorContext(ctx, "outbox: dispatcher: ClaimReady falhou", slog.String("error", err.Error()))
		return
	}
	now := d.clock.Now()
	d.recordPoll(ctx, now.Sub(startedAt), len(claims))
	d.refreshPendingIfDue(ctx, now)

	for _, claim := range claims {
		d.wg.Go(func() {
			d.deliver(ctx, claim)
		})
	}
}

// deliver hidrata o handler via Registry, envolve em timeout e executa.
// Classifica o resultado via markResult.
func (d *Dispatcher) deliver(ctx context.Context, claim Claim) {
	ctx = contextWithEventTraceparent(ctx, claim.Event.Headers())
	ctx, deliverSpan := d.startDeliverySpan(ctx, claim)

	subs := d.registry.SubscriptionsFor(claim.Event.Type())
	if len(subs) == 0 {
		d.logger.WarnContext(ctx, "outbox: dispatcher: nenhum handler para event_type",
			slog.String("event_type", claim.Event.Type().String()),
			slog.Int64("claim_id", int64(claim.ID)),
		)
		d.endDeliverySpan(deliverSpan, ErrPermanent)
		d.markResult(ctx, claim, ErrPermanent)
		return
	}

	// Encontra a subscription correspondente ao nome desta delivery.
	var handler Handler
	for _, sub := range subs {
		if sub.Name == claim.SubscriptionName {
			handler = sub.Handler
			break
		}
	}

	if handler == nil {
		d.logger.WarnContext(ctx, "outbox: dispatcher: subscription nao encontrada",
			slog.String("subscription_name", claim.SubscriptionName.String()),
			slog.Int64("claim_id", int64(claim.ID)),
		)
		d.endDeliverySpan(deliverSpan, ErrPermanent)
		d.markResult(ctx, claim, ErrPermanent)
		return
	}

	handlerCtx, cancel := context.WithTimeout(ctx, d.handlerTimeout)
	defer cancel()

	handlerCtx, handlerSpan := d.startHandlerSpan(handlerCtx, claim)
	err := d.runHandler(handlerCtx, handler, claim.Event)
	d.endDeliverySpan(handlerSpan, err)
	d.endDeliverySpan(deliverSpan, err)
	d.markResult(ctx, claim, err)
}

func contextWithEventTraceparent(ctx context.Context, headers Headers) context.Context {
	traceparent, ok := headers.Get("traceparent")
	if !ok || traceparent == "" {
		return ctx
	}
	return propagation.TraceContext{}.Extract(ctx, propagation.MapCarrier{"traceparent": traceparent})
}

// runHandler executa o handler capturando panics como ErrPermanent.
func (d *Dispatcher) runHandler(ctx context.Context, handler Handler, evt Event) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("outbox: handler panic: %v: %w", r, ErrPermanent)
		}
	}()
	return handler(ctx, evt)
}

// markResult classifica o erro e chama o método de Storage correspondente.
// Emite logs estruturados via logFields (RF-23, RF-31). Payload JAMAIS aparece (RF-24, R-SEC-001).
// O log outbox.delivery.processed é amostrado 1:100 via contador atômico (RF-36, subtarefa 9.3).
func (d *Dispatcher) markResult(ctx context.Context, claim Claim, handlerErr error) {
	now := d.clock.Now()
	start := claim.ClaimedAt

	if handlerErr == nil {
		if err := d.storage.MarkProcessed(ctx, claim.ID, now); err != nil {
			d.logger.ErrorContext(ctx, "outbox: dispatcher: MarkProcessed falhou",
				slog.Int64("claim_id", int64(claim.ID)),
				slog.String("error", err.Error()),
			)
		}
		d.recordDeliverySuccess(ctx, claim, now)

		// Sampler 1:100: emite log INFO apenas a cada 100 entregas processadas (subtarefa 9.3).
		n := d.processedCounter.Add(1)
		if n%100 == 0 {
			fields := logFieldsFromClaim(claim)
			if !start.IsZero() {
				fields.LatencyMs = now.Sub(start).Milliseconds()
			}
			d.logger.LogAttrs(ctx, slog.LevelInfo, "outbox.delivery.processed", fields.Attrs()...)
		}
		return
	}

	// ErrPermanent → DLQ imediato.
	if errors.Is(handlerErr, ErrPermanent) {
		if err := d.storage.MarkDLQ(ctx, claim.ID, handlerErr.Error(), now); err != nil {
			d.logger.ErrorContext(ctx, "outbox: dispatcher: MarkDLQ (permanent) falhou",
				slog.Int64("claim_id", int64(claim.ID)),
				slog.String("error", err.Error()),
			)
		}
		d.recordDeliveryDLQ(ctx, claim.SubscriptionName.String())

		// ERROR outbox.delivery.dlq — sempre logado (sem sampler).
		fields := logFieldsFromClaim(claim)
		fields.ErrorClass = classifyError(handlerErr)
		fields.TotalAttempts = claim.Attempt.Value()
		d.logger.LogAttrs(ctx, slog.LevelError, "outbox.delivery.dlq", fields.Attrs()...)
		return
	}

	// Attempts esgotados → DLQ.
	if claim.Attempt.IsExhausted(d.maxAttempts) {
		if err := d.storage.MarkDLQ(ctx, claim.ID, handlerErr.Error(), now); err != nil {
			d.logger.ErrorContext(ctx, "outbox: dispatcher: MarkDLQ (exhausted) falhou",
				slog.Int64("claim_id", int64(claim.ID)),
				slog.String("error", err.Error()),
			)
		}
		d.recordDeliveryDLQ(ctx, claim.SubscriptionName.String())

		// ERROR outbox.delivery.dlq — sempre logado (sem sampler).
		fields := logFieldsFromClaim(claim)
		fields.ErrorClass = classifyError(handlerErr)
		fields.TotalAttempts = claim.Attempt.Value()
		d.logger.LogAttrs(ctx, slog.LevelError, "outbox.delivery.dlq", fields.Attrs()...)
		return
	}

	// Falha transitória (inclui DeadlineExceeded) → MarkFailed com nextRetryAt.
	nextAttempt := claim.Attempt
	nextRetryAt := d.policy.NextRetryAt(claim.Attempt, now)

	if err := d.storage.MarkFailed(ctx, claim.ID, handlerErr.Error(), nextAttempt, nextRetryAt); err != nil {
		d.logger.ErrorContext(ctx, "outbox: dispatcher: MarkFailed falhou",
			slog.Int64("claim_id", int64(claim.ID)),
			slog.String("error", err.Error()),
		)
	}
	d.recordDeliveryFailure(ctx, claim.SubscriptionName.String(), handlerErr)

	// WARN outbox.delivery.failed — sempre logado (sem sampler).
	fields := logFieldsFromClaim(claim)
	fields.ErrorClass = classifyError(handlerErr)
	fields.NextRetryAt = nextRetryAt
	d.logger.LogAttrs(ctx, slog.LevelWarn, "outbox.delivery.failed", fields.Attrs()...)
}

func (d *Dispatcher) startDeliverySpan(ctx context.Context, claim Claim) (context.Context, observability.Span) {
	if d.tracer == nil {
		return ctx, nil
	}
	return d.tracer.Start(ctx, "outbox.deliver",
		observability.WithSpanKind(observability.SpanKindConsumer),
		observability.WithAttributes(
			observability.String("event_id", claim.Event.ID().String()),
			observability.String("event_type", claim.Event.Type().String()),
			observability.String("subscription_name", claim.SubscriptionName.String()),
		),
	)
}

func (d *Dispatcher) startHandlerSpan(ctx context.Context, claim Claim) (context.Context, observability.Span) {
	if d.tracer == nil {
		return ctx, nil
	}
	return d.tracer.Start(ctx, "outbox.handle."+claim.SubscriptionName.String(),
		observability.WithSpanKind(observability.SpanKindConsumer),
		observability.WithAttributes(
			observability.String("event_id", claim.Event.ID().String()),
			observability.String("event_type", claim.Event.Type().String()),
			observability.String("subscription_name", claim.SubscriptionName.String()),
		),
	)
}

func (d *Dispatcher) endDeliverySpan(span observability.Span, err error) {
	if span == nil {
		return
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(observability.StatusCodeError, classifyError(err))
		span.End()
		return
	}
	span.SetStatus(observability.StatusCodeOK, "")
	span.End()
}

func (d *Dispatcher) recordPoll(ctx context.Context, duration time.Duration, batchSize int) {
	recorder, ok := d.metrics.(pollMetricsRecorder)
	if !ok {
		return
	}
	if duration < 0 {
		duration = 0
	}
	recorder.recordPoll(ctx, duration, batchSize)
}

func (d *Dispatcher) refreshPendingIfDue(ctx context.Context, now time.Time) {
	recorder, ok := d.metrics.(pendingMetricsRecorder)
	if !ok {
		return
	}

	d.pendingMu.Lock()
	if !d.lastStatsAt.IsZero() && now.Sub(d.lastStatsAt) < 30*time.Second {
		d.pendingMu.Unlock()
		return
	}
	d.lastStatsAt = now
	d.pendingMu.Unlock()

	stats, err := d.storage.Stats(ctx)
	if err != nil {
		d.logger.WarnContext(ctx, "outbox: dispatcher: Stats falhou", slog.String("error", err.Error()))
		return
	}

	d.pendingMu.Lock()
	defer d.pendingMu.Unlock()
	seen := make(map[string]struct{}, len(stats.Pending))
	for subscriptionName, count := range stats.Pending {
		name := subscriptionName.String()
		seen[name] = struct{}{}
		previous := d.pendingCounts[name]
		delta := count - previous
		if delta == 0 {
			continue
		}
		d.pendingCounts[name] = count
		recorder.setPendingDelta(ctx, name, delta)
	}
	for name, previous := range d.pendingCounts {
		if _, ok := seen[name]; ok || previous == 0 {
			continue
		}
		d.pendingCounts[name] = 0
		recorder.setPendingDelta(ctx, name, -previous)
	}
}

func (d *Dispatcher) recordDeliverySuccess(ctx context.Context, claim Claim, now time.Time) {
	subscriptionName := claim.SubscriptionName.String()
	latency := now.Sub(claim.Event.OccurredAt())
	if latency < 0 {
		latency = 0
	}
	if recorder, ok := d.metrics.(deliveryMetricsRecorder); ok {
		recorder.recordDeliverySuccess(ctx, subscriptionName, float64(latency.Milliseconds()))
		return
	}
	d.metrics.RecordDeliverySuccess(ctx, subscriptionName)
}

func (d *Dispatcher) recordDeliveryFailure(ctx context.Context, subscriptionName string, err error) {
	if recorder, ok := d.metrics.(deliveryMetricsRecorder); ok {
		recorder.recordDeliveryFailure(ctx, subscriptionName, err)
		return
	}
	d.metrics.RecordDeliveryFailure(ctx, subscriptionName)
}

func (d *Dispatcher) recordDeliveryDLQ(ctx context.Context, subscriptionName string) {
	if recorder, ok := d.metrics.(deliveryMetricsRecorder); ok {
		recorder.recordDeliveryDLQ(ctx, subscriptionName)
		return
	}
	d.metrics.RecordDeliveryDLQ(ctx, subscriptionName)
}
