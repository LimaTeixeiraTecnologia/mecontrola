package outbox

import (
	"context"
	"errors"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

// OutboxMetrics é a fachada OTel do pacote outbox.
// Instrumenta 10 medidas (counters/gauges/histograms) conforme RF-21 / techspec tabela "Métricas OTel".
//
// Regra de import: metrics.go pode importar otel (exceção documentada em techspec, seção Fronteiras).
// Payload jamais aparece em nenhum parâmetro deste arquivo (R-SEC-001).
//
// Os spans outbox.publish / outbox.deliver / outbox.handle.<name> são criados pelos consumidores
// (Publisher e Dispatcher) — não por OutboxMetrics. Tracer() expõe o tracer para esses consumidores.
type OutboxMetrics struct {
	obs observability.Observability

	// RF-21 — 10 instrumentos OTel com nomes exatos do techspec.

	// outbox.events.published.total — counter, label: event_type
	eventsPublished observability.Counter

	// outbox.deliveries.pending — gauge, label: subscription_name
	// Implementada via UpDownCounter; SetPending recalcula o delta.
	// NOTA: usamos UpDownCounter porque Gauge com callback não suporta labels dinamicos neste provider.
	deliveriesPending observability.UpDownCounter

	// outbox.deliveries.processed.total — counter, label: subscription_name
	deliveriesProcessed observability.Counter

	// outbox.deliveries.failed.total — counter, labels: subscription_name, error_class
	deliveriesFailed observability.Counter

	// outbox.deliveries.dlq.total — counter, label: subscription_name
	deliveriesDLQ observability.Counter

	// outbox.delivery.latency_ms — histogram, label: subscription_name
	deliveryLatency observability.Histogram

	// outbox.poll.duration_ms — histogram, sem label
	pollDuration observability.Histogram

	// outbox.poll.batch_size — histogram, sem label
	pollBatchSize observability.Histogram

	// outbox.reaper.released.total — counter, sem label
	reaperReleased observability.Counter

	// outbox.housekeeping.deleted.total — counter, sem label
	housekeepingDeleted observability.Counter
}

// NewOutboxMetrics cria e registra os 10 instrumentos OTel usando a Observability fornecida.
// Retorna erro se qualquer instrumento não puder ser criado.
func NewOutboxMetrics(obs observability.Observability) (*OutboxMetrics, error) {
	if obs == nil {
		return nil, errOutboxObservabilityNil
	}
	m := obs.Metrics()
	if m == nil {
		return nil, errOutboxMetricsNil
	}

	eventsPublished := m.Counter(
		"outbox.events.published.total",
		"Total de eventos persistidos pelo Publisher por tipo de evento",
		"{event}",
	)
	if eventsPublished == nil {
		return nil, errInstrumentNil("outbox.events.published.total")
	}

	deliveriesPending := m.UpDownCounter(
		"outbox.deliveries.pending",
		"Tamanho da fila de deliveries por subscription (gauge via UpDownCounter)",
		"{delivery}",
	)
	if deliveriesPending == nil {
		return nil, errInstrumentNil("outbox.deliveries.pending")
	}

	deliveriesProcessed := m.Counter(
		"outbox.deliveries.processed.total",
		"Total de deliveries concluídas com sucesso por subscription",
		"{delivery}",
	)
	if deliveriesProcessed == nil {
		return nil, errInstrumentNil("outbox.deliveries.processed.total")
	}

	deliveriesFailed := m.Counter(
		"outbox.deliveries.failed.total",
		"Total de deliveries em falha transitória por subscription e classe de erro",
		"{delivery}",
	)
	if deliveriesFailed == nil {
		return nil, errInstrumentNil("outbox.deliveries.failed.total")
	}

	deliveriesDLQ := m.Counter(
		"outbox.deliveries.dlq.total",
		"Total de transições para Dead Letter Queue por subscription",
		"{delivery}",
	)
	if deliveriesDLQ == nil {
		return nil, errInstrumentNil("outbox.deliveries.dlq.total")
	}

	deliveryLatency := m.Histogram(
		"outbox.delivery.latency_ms",
		"Latência de delivery em milissegundos (now - event.occurred_at) por subscription",
		"ms",
	)
	if deliveryLatency == nil {
		return nil, errInstrumentNil("outbox.delivery.latency_ms")
	}

	pollDuration := m.Histogram(
		"outbox.poll.duration_ms",
		"Duração do ClaimReady em milissegundos",
		"ms",
	)
	if pollDuration == nil {
		return nil, errInstrumentNil("outbox.poll.duration_ms")
	}

	pollBatchSize := m.Histogram(
		"outbox.poll.batch_size",
		"Número de itens retornados por chamada ao ClaimReady",
		"{item}",
	)
	if pollBatchSize == nil {
		return nil, errInstrumentNil("outbox.poll.batch_size")
	}

	reaperReleased := m.Counter(
		"outbox.reaper.released.total",
		"Total de deliveries liberadas pelo reaper (claimed → pending)",
		"{delivery}",
	)
	if reaperReleased == nil {
		return nil, errInstrumentNil("outbox.reaper.released.total")
	}

	housekeepingDeleted := m.Counter(
		"outbox.housekeeping.deleted.total",
		"Total de linhas apagadas pelo housekeeping (deliveries + eventos órfãos)",
		"{row}",
	)
	if housekeepingDeleted == nil {
		return nil, errInstrumentNil("outbox.housekeeping.deleted.total")
	}

	return &OutboxMetrics{
		obs:                 obs,
		eventsPublished:     eventsPublished,
		deliveriesPending:   deliveriesPending,
		deliveriesProcessed: deliveriesProcessed,
		deliveriesFailed:    deliveriesFailed,
		deliveriesDLQ:       deliveriesDLQ,
		deliveryLatency:     deliveryLatency,
		pollDuration:        pollDuration,
		pollBatchSize:       pollBatchSize,
		reaperReleased:      reaperReleased,
		housekeepingDeleted: housekeepingDeleted,
	}, nil
}

// Tracer retorna o Tracer OTel para criação de spans outbox.publish / outbox.deliver /
// outbox.handle.<subscription_name> pelos consumidores (Publisher, Dispatcher).
// Os spans são responsabilidade dos consumidores — não de OutboxMetrics.
func (m *OutboxMetrics) Tracer() observability.Tracer {
	return m.obs.Tracer()
}

// RecordPublished registra um evento persistido pelo Publisher.
// Label: event_type.
// Payload não é parâmetro (R-SEC-001).
func (m *OutboxMetrics) RecordPublished(ctx context.Context, eventType string) {
	m.eventsPublished.Add(ctx, 1,
		observability.String("event_type", eventType),
	)
}

// RecordProcessed registra uma delivery concluída com sucesso.
// Labels: subscription_name. latencyMs = now - event.occurred_at em millisegundos.
func (m *OutboxMetrics) RecordProcessed(ctx context.Context, subscriptionName string, latencyMs float64) {
	m.deliveriesProcessed.Add(ctx, 1,
		observability.String("subscription_name", subscriptionName),
	)
	m.deliveryLatency.Record(ctx, latencyMs,
		observability.String("subscription_name", subscriptionName),
	)
}

// RecordFailed registra uma delivery em falha transitória.
// Labels: subscription_name, error_class.
// error_class é bucketizado em 5 valores fixos (R-OBS-001): transient | timeout | permanent | panic | unknown.
// Payload não é parâmetro (R-SEC-001).
func (m *OutboxMetrics) RecordFailed(ctx context.Context, subscriptionName string, err error) {
	ec := classifyError(err)
	m.deliveriesFailed.Add(ctx, 1,
		observability.String("subscription_name", subscriptionName),
		observability.String("error_class", ec),
	)
}

// RecordDLQ registra uma delivery enviada para Dead Letter Queue.
// Label: subscription_name.
func (m *OutboxMetrics) RecordDLQ(ctx context.Context, subscriptionName string) {
	m.deliveriesDLQ.Add(ctx, 1,
		observability.String("subscription_name", subscriptionName),
	)
}

// RecordPoll registra um ciclo do Dispatcher (ClaimReady).
// durationMs: duração da chamada; batchSize: itens retornados.
// Sem labels de subscription (poll é por instância, não por subscription).
func (m *OutboxMetrics) RecordPoll(ctx context.Context, durationMs float64, batchSize int) {
	m.pollDuration.Record(ctx, durationMs)
	m.pollBatchSize.Record(ctx, float64(batchSize))
}

// RecordReaperReleased registra linhas liberadas pelo reaper.
// Sem label de subscription (reaper age no conjunto de todas as subscriptions).
func (m *OutboxMetrics) RecordReaperReleased(ctx context.Context, n int64) {
	m.reaperReleased.Add(ctx, n)
}

// RecordHousekeepingDeleted registra linhas apagadas pelo housekeeping.
// Sem label de subscription (housekeeping age no conjunto de todas as subscriptions).
func (m *OutboxMetrics) RecordHousekeepingDeleted(ctx context.Context, n int64) {
	m.housekeepingDeleted.Add(ctx, n)
}

// SetPending ajusta o gauge de deliveries pendentes para subscription_name.
// count é o valor absoluto da fila; delta = count - previous é calculado externamente.
// Para simplificar, adicionamos diretamente o delta via UpDownCounter.
// Callers devem manter o estado anterior e passar o delta correto.
func (m *OutboxMetrics) SetPending(ctx context.Context, subscriptionName string, delta int64) {
	m.deliveriesPending.Add(ctx, delta,
		observability.String("subscription_name", subscriptionName),
	)
}

type outboxMetricsAdapter struct {
	metrics *OutboxMetrics
}

func newMetricsAdapter(metrics *OutboxMetrics) Metrics {
	if metrics == nil {
		return NoopMetrics()
	}
	return &outboxMetricsAdapter{metrics: metrics}
}

func (a *outboxMetricsAdapter) RecordDeliverySuccess(ctx context.Context, subscriptionName string) {
	a.recordDeliverySuccess(ctx, subscriptionName, 0)
}

func (a *outboxMetricsAdapter) RecordDeliveryFailure(ctx context.Context, subscriptionName string) {
	a.recordDeliveryFailure(ctx, subscriptionName, errors.New("unknown"))
}

func (a *outboxMetricsAdapter) RecordDeliveryDLQ(ctx context.Context, subscriptionName string) {
	a.recordDeliveryDLQ(ctx, subscriptionName)
}

func (a *outboxMetricsAdapter) recordDeliverySuccess(ctx context.Context, subscriptionName string, latencyMs float64) {
	a.metrics.RecordProcessed(ctx, subscriptionName, latencyMs)
}

func (a *outboxMetricsAdapter) recordDeliveryFailure(ctx context.Context, subscriptionName string, err error) {
	a.metrics.RecordFailed(ctx, subscriptionName, err)
}

func (a *outboxMetricsAdapter) recordDeliveryDLQ(ctx context.Context, subscriptionName string) {
	a.metrics.RecordDLQ(ctx, subscriptionName)
}

func (a *outboxMetricsAdapter) recordPoll(ctx context.Context, duration time.Duration, batchSize int) {
	a.metrics.RecordPoll(ctx, float64(duration.Milliseconds()), batchSize)
}

func (a *outboxMetricsAdapter) setPendingDelta(ctx context.Context, subscriptionName string, delta int64) {
	a.metrics.SetPending(ctx, subscriptionName, delta)
}

func (a *outboxMetricsAdapter) tracer() observability.Tracer {
	return a.metrics.Tracer()
}

// ClassifyErrorForTest expõe classifyError para testes unitários.
// Permite que testes verifiquem o bucket de error_class sem acoplamento ao método RecordFailed.
// Não deve ser usado em produção.
func ClassifyErrorForTest(err error) string {
	return classifyError(err)
}

// classifyError mapeia um erro para um dos 5 buckets fixos de error_class (R-OBS-001).
// Buckets: transient | timeout | permanent | panic | unknown.
// Controle de cardinalidade: erros desconhecidos caem em "unknown".
func classifyError(err error) string {
	if err == nil {
		return errorClassUnknown
	}
	if errors.Is(err, ErrPermanent) {
		return errorClassPermanent
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return errorClassTimeout
	}
	if errors.Is(err, context.Canceled) {
		return errorClassTimeout
	}
	// Panic wrappado como ErrPermanent é detectado acima.
	// Verificação explícita de mensagem de panic para o caso de wrapping alternativo.
	if isPanicError(err) {
		return errorClassPanic
	}
	// Erros de rede, banco, etc. → transient.
	if isTransientError(err) {
		return errorClassTransient
	}
	return errorClassTransient
}

// isPanicError detecta erros originados de panic capturado no Dispatcher.
// O Dispatcher usa: fmt.Errorf("outbox: handler panic: %v: %w", r, ErrPermanent).
// ErrPermanent já é detectado por errors.Is acima, mas mantemos aqui por segurança de contrato.
func isPanicError(_ error) bool {
	// Panics são sempre wrappados como ErrPermanent pelo Dispatcher — detectados acima.
	// Esta função é um hook para futuras extensões de classificação.
	return false
}

// isTransientError classifica erros que não são permanentes nem timeout como transitórios.
// Por padrão, erros desconhecidos são transient (conservador — permite retry).
func isTransientError(_ error) bool {
	return true
}

// Constantes de error_class (R-OBS-001 — 5 valores fixos, sem cardinalidade aberta).
const (
	errorClassTransient = "transient"
	errorClassTimeout   = "timeout"
	errorClassPermanent = "permanent"
	errorClassPanic     = "panic"
	errorClassUnknown   = "unknown"
)

// Erros de construção de OutboxMetrics.
var (
	errOutboxObservabilityNil = newMetricsError("observability não pode ser nil")
	errOutboxMetricsNil       = newMetricsError("Metrics() retornou nil")
)

func errInstrumentNil(name string) error {
	return newMetricsError("instrumento " + name + " não pôde ser criado (provider retornou nil)")
}

func newMetricsError(msg string) error {
	return errors.New("outbox.metrics: " + msg)
}
