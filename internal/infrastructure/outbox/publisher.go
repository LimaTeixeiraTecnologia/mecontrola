package outbox

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
)

// Publisher é a API de superfície invocada pelo use case dentro de um UnitOfWork[T].Do
// ativo. Insere evento + N deliveries na mesma tx recebida, sem abrir transação própria,
// sem retry, sem rede. O commit é responsabilidade do caller.
//
// Regra de import: publisher.go NÃO importa pgx. Toda persistência passa por Storage.
type Publisher interface {
	// Publish persiste evt e uma delivery por cada subscription registrada para evt.Type()
	// dentro da transação tx fornecida pelo UnitOfWork[T].
	//
	// Retorna ErrHandlerNotRegistered se nenhuma subscription estiver registrada.
	// Qualquer erro de Storage é retornado wrappado com prefixo "outbox.publish:".
	Publish(ctx context.Context, tx database.DBTX, evt Event) error
}

// transactionalPublisher implementa Publisher usando Storage e Registry como colaboradores.
// Tem exatamente dois colaboradores (OC #7): storage e registry. Tracer é opcional — se nil,
// usa noop (sem custo, sem panic).
type transactionalPublisher struct {
	storage  Storage
	registry Registry
	tracer   trace.Tracer
	prop     propagation.TextMapPropagator
	metrics  *OutboxMetrics
}

// NewPublisher cria um Publisher transacional.
// tracer é opcional: nil resulta em noop tracer (sem custo de rede, sem panic).
func NewPublisher(storage Storage, registry Registry, tracer trace.Tracer) Publisher {
	return NewPublisherWithMetrics(storage, registry, tracer, nil)
}

// NewPublisherWithMetrics cria um Publisher transacional instrumentado.
// metrics é opcional: nil preserva o comportamento sem métricas.
func NewPublisherWithMetrics(storage Storage, registry Registry, tracer trace.Tracer, metrics *OutboxMetrics) Publisher {
	if tracer == nil {
		tracer = noop.NewTracerProvider().Tracer("outbox")
	}
	return &transactionalPublisher{
		storage:  storage,
		registry: registry,
		tracer:   tracer,
		prop:     propagation.TraceContext{},
		metrics:  metrics,
	}
}

// Publish implementa Publisher.
//
// Fluxo:
//  1. Inicia span "outbox.publish" (kind INTERNAL) com atributos event_id e event_type.
//  2. Injeta traceparent em evt.Headers via W3C TraceContext propagator.
//  3. Consulta Registry.SubscriptionsFor — retorna ErrHandlerNotRegistered se vazio.
//  4. Chama Storage.InsertEvent na tx recebida.
//  5. Chama Storage.InsertDeliveries (lote único de SubscriptionNames) na mesma tx.
//
// Nenhum passo abre nova transação. O commit é responsabilidade do caller (UnitOfWork).
func (p *transactionalPublisher) Publish(ctx context.Context, tx database.DBTX, evt Event) error {
	ctx, span := p.tracer.Start(ctx, "outbox.publish",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("event_id", evt.ID().String()),
			attribute.String("event_type", evt.Type().String()),
		),
	)
	defer span.End()

	// Injeta traceparent em Headers antes dos inserts (RF-22, cenário 6).
	carrier := make(propagation.MapCarrier)
	p.prop.Inject(ctx, carrier)
	if tp, ok := carrier["traceparent"]; ok && tp != "" {
		evt.headers = evt.headers.WithTrace(tp)
	}

	subs := p.registry.SubscriptionsFor(evt.Type())
	if len(subs) == 0 {
		return ErrHandlerNotRegistered
	}

	if err := p.storage.InsertEvent(ctx, tx, evt); err != nil {
		return fmt.Errorf("outbox.publish: %w", err)
	}

	names := extractNames(subs)
	if err := p.storage.InsertDeliveries(ctx, tx, evt.ID(), names); err != nil {
		return fmt.Errorf("outbox.publish: %w", err)
	}

	if p.metrics != nil {
		p.metrics.RecordPublished(ctx, evt.Type().String())
	}

	return nil
}

// extractNames extrai os SubscriptionName de uma slice de Subscription.
// Callers devem garantir que subs não é vazio antes de chamar (Publish verifica via len(subs)==0).
func extractNames(subs []Subscription) []SubscriptionName {
	names := make([]SubscriptionName, len(subs))
	for i, s := range subs {
		names[i] = s.Name
	}
	return names
}
