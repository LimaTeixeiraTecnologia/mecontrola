package outbox_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	obsfake "github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/outbox/mocks"
)

// stubDBTX é uma implementação mínima de database.DBTX para testes.
// Permite verificar que a mesma instância de tx é passada para InsertEvent e InsertDeliveries
// sem precisar de um banco real.
type stubDBTX struct{}

func (s *stubDBTX) ExecContext(_ context.Context, _ string, _ ...any) (database.Result, error) {
	return nil, nil
}

func (s *stubDBTX) QueryContext(_ context.Context, _ string, _ ...any) (database.Rows, error) {
	return nil, nil
}

func (s *stubDBTX) QueryRowContext(_ context.Context, _ string, _ ...any) database.Row {
	return nil
}

// PublisherSuite testa o Publisher transacional com todos os cenários obrigatórios da task 5.0.
type PublisherSuite struct {
	suite.Suite

	ctx      context.Context
	storage  *mocks.Storage
	registry *mocks.Registry
	tx       database.DBTX

	validID   events.EventID
	validType events.EventName
	validEvt  outbox.Event

	sub1 outbox.Subscription
	sub2 outbox.Subscription
	sub3 outbox.Subscription
}

func TestPublisher(t *testing.T) {
	suite.Run(t, new(PublisherSuite))
}

func (s *PublisherSuite) SetupTest() {
	s.ctx = context.Background()
	s.storage = mocks.NewStorage(s.T())
	s.registry = mocks.NewRegistry(s.T())
	s.tx = &stubDBTX{}

	id, err := events.NewEventID("01ARZ3NDEKTSV4RRFFQ69G5FAV")
	s.Require().NoError(err)
	s.validID = id

	name, err := events.NewEventName("identity.user-created")
	s.Require().NoError(err)
	s.validType = name

	evt, err := outbox.NewEvent(outbox.NewEventParams{
		ID:            s.validID,
		EventType:     s.validType,
		AggregateType: "user",
		AggregateID:   "u-123",
		Payload:       json.RawMessage(`{"ok":true}`),
	})
	s.Require().NoError(err)
	s.validEvt = evt

	mkSub := func(name string) outbox.Subscription {
		sn, err := outbox.NewSubscriptionName(name)
		s.Require().NoError(err)
		return outbox.Subscription{
			Name:      sn,
			EventType: s.validType,
			Handler:   func(_ context.Context, _ outbox.Event) error { return nil },
		}
	}
	s.sub1 = mkSub("identity-handler")
	s.sub2 = mkSub("audit-handler")
	s.sub3 = mkSub("notify-handler")
}

// Cenário 1: 1 handler registrado → 1 InsertEvent + 1 InsertDeliveries com names=[s1].
// Nenhuma transação criada internamente (publisher recebe tx e repassa sem abrir nova).
func (s *PublisherSuite) TestPublish_OneHandler_Success() {
	subs := []outbox.Subscription{s.sub1}
	names := []outbox.SubscriptionName{s.sub1.Name}

	s.registry.EXPECT().SubscriptionsFor(s.validType).Return(subs)
	s.storage.EXPECT().InsertEvent(mock.Anything, s.tx, mock.Anything).Return(nil)
	s.storage.EXPECT().InsertDeliveries(mock.Anything, s.tx, s.validID, names).Return(nil)

	pub := outbox.NewPublisher(s.storage, s.registry, nil)
	err := pub.Publish(s.ctx, s.tx, s.validEvt)
	s.NoError(err)
}

// Cenário 2: 3 handlers → 1 InsertEvent + 1 InsertDeliveries com names=[s1,s2,s3] (lote único).
func (s *PublisherSuite) TestPublish_ThreeHandlers_SingleBatch() {
	subs := []outbox.Subscription{s.sub1, s.sub2, s.sub3}
	names := []outbox.SubscriptionName{s.sub1.Name, s.sub2.Name, s.sub3.Name}

	s.registry.EXPECT().SubscriptionsFor(s.validType).Return(subs)
	s.storage.EXPECT().InsertEvent(mock.Anything, s.tx, mock.Anything).Return(nil)
	s.storage.EXPECT().InsertDeliveries(mock.Anything, s.tx, s.validID, names).Return(nil)

	pub := outbox.NewPublisher(s.storage, s.registry, nil)
	err := pub.Publish(s.ctx, s.tx, s.validEvt)
	s.NoError(err)
}

// Cenário 3: SubscriptionsFor retorna vazio → ErrHandlerNotRegistered; nenhum insert.
func (s *PublisherSuite) TestPublish_NoSubscription_ErrHandlerNotRegistered() {
	s.registry.EXPECT().SubscriptionsFor(s.validType).Return(nil)
	// storage NÃO deve ser chamado — mocks.NewStorage registra AssertExpectations automático.

	pub := outbox.NewPublisher(s.storage, s.registry, nil)
	err := pub.Publish(s.ctx, s.tx, s.validEvt)
	s.Error(err)
	s.True(errors.Is(err, outbox.ErrHandlerNotRegistered),
		"esperado ErrHandlerNotRegistered, recebido: %v", err)
}

// Cenário 4: InsertEvent falha → erro retornado wrappado com prefixo "outbox.publish:".
func (s *PublisherSuite) TestPublish_InsertEventFails_WrappedError() {
	storageErr := errors.New("db is down")
	subs := []outbox.Subscription{s.sub1}

	s.registry.EXPECT().SubscriptionsFor(s.validType).Return(subs)
	s.storage.EXPECT().InsertEvent(mock.Anything, s.tx, mock.Anything).Return(storageErr)
	// InsertDeliveries NÃO deve ser chamado após InsertEvent falhar.

	pub := outbox.NewPublisher(s.storage, s.registry, nil)
	err := pub.Publish(s.ctx, s.tx, s.validEvt)
	s.Error(err)
	s.True(errors.Is(err, storageErr), "erro original deve ser unwrappável via errors.Is")
	s.Contains(err.Error(), "outbox.publish:", "mensagem deve ter prefixo outbox.publish:")
}

// Cenário 4b: InsertDeliveries falha → erro retornado wrappado.
func (s *PublisherSuite) TestPublish_InsertDeliveriesFails_WrappedError() {
	storageErr := errors.New("deliveries insert failed")
	subs := []outbox.Subscription{s.sub1}
	names := []outbox.SubscriptionName{s.sub1.Name}

	s.registry.EXPECT().SubscriptionsFor(s.validType).Return(subs)
	s.storage.EXPECT().InsertEvent(mock.Anything, s.tx, mock.Anything).Return(nil)
	s.storage.EXPECT().InsertDeliveries(mock.Anything, s.tx, s.validID, names).Return(storageErr)

	pub := outbox.NewPublisher(s.storage, s.registry, nil)
	err := pub.Publish(s.ctx, s.tx, s.validEvt)
	s.Error(err)
	s.True(errors.Is(err, storageErr), "erro original deve ser unwrappável via errors.Is")
	s.Contains(err.Error(), "outbox.publish:", "mensagem deve ter prefixo outbox.publish:")
}

// Cenário 5: a tx recebida é exatamente a mesma instância passada para InsertEvent e InsertDeliveries.
// Garante que Publisher não abre nova transação internamente (RF-01, subtarefa 5.3).
func (s *PublisherSuite) TestPublish_TxIsPreserved_SameInstance() {
	subs := []outbox.Subscription{s.sub1}
	names := []outbox.SubscriptionName{s.sub1.Name}

	var capturedTxEvent database.DBTX
	var capturedTxDelivery database.DBTX

	s.registry.EXPECT().SubscriptionsFor(s.validType).Return(subs)
	s.storage.EXPECT().InsertEvent(mock.Anything, s.tx, mock.Anything).
		Run(func(_ context.Context, tx database.DBTX, _ outbox.Event) {
			capturedTxEvent = tx
		}).
		Return(nil)
	s.storage.EXPECT().InsertDeliveries(mock.Anything, s.tx, s.validID, names).
		Run(func(_ context.Context, tx database.DBTX, _ events.EventID, _ []outbox.SubscriptionName) {
			capturedTxDelivery = tx
		}).
		Return(nil)

	pub := outbox.NewPublisher(s.storage, s.registry, nil)
	err := pub.Publish(s.ctx, s.tx, s.validEvt)
	s.NoError(err)

	// Assert de ponteiro: mesma instância de tx.
	s.Same(s.tx.(*stubDBTX), capturedTxEvent.(*stubDBTX),
		"InsertEvent deve receber exatamente a mesma instância de tx")
	s.Same(s.tx.(*stubDBTX), capturedTxDelivery.(*stubDBTX),
		"InsertDeliveries deve receber exatamente a mesma instância de tx")
}

// Cenário 6: span "outbox.publish" criado e traceparent injetado em evt.Headers antes dos inserts.
// Verifica via sdktrace.InMemoryExporter (tracetest.SpanRecorder equivalente).
func (s *PublisherSuite) TestPublish_SpanCreated_TraceParentInjected() {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	defer func() { _ = tp.Shutdown(s.ctx) }()

	tracer := tp.Tracer("outbox-test")
	pub := outbox.NewPublisher(s.storage, s.registry, tracer)

	subs := []outbox.Subscription{s.sub1}
	names := []outbox.SubscriptionName{s.sub1.Name}

	// Captura o evt recebido pelo InsertEvent (após injeção do traceparent).
	var capturedEvt outbox.Event

	s.registry.EXPECT().SubscriptionsFor(s.validType).Return(subs)
	s.storage.EXPECT().InsertEvent(
		mock.Anything,
		s.tx,
		mock.MatchedBy(func(e outbox.Event) bool {
			capturedEvt = e
			return true
		}),
	).Return(nil)
	s.storage.EXPECT().InsertDeliveries(mock.Anything, s.tx, s.validID, names).Return(nil)

	err := pub.Publish(s.ctx, s.tx, s.validEvt)
	s.NoError(err)

	// Verificar span criado com nome correto.
	spans := exporter.GetSpans()
	s.Require().Len(spans, 1, "deve haver exatamente 1 span")
	s.Equal("outbox.publish", spans[0].Name, "span deve ter nome outbox.publish")

	// Verificar traceparent injetado em Headers do evt antes do InsertEvent.
	traceparent, ok := capturedEvt.Headers().Get("traceparent")
	s.True(ok, "traceparent deve estar presente nos Headers do evt passado para InsertEvent")
	s.NotEmpty(traceparent, "traceparent nao pode ser vazio")

	// Verificar atributos event_id e event_type no span.
	attrs := spans[0].Attributes
	s.Require().NotEmpty(attrs, "span deve ter atributos")
	attrMap := make(map[string]string, len(attrs))
	for _, a := range attrs {
		attrMap[string(a.Key)] = a.Value.AsString()
	}
	s.Equal(s.validID.String(), attrMap["event_id"], "span deve ter atributo event_id correto")
	s.Equal(s.validType.String(), attrMap["event_type"], "span deve ter atributo event_type correto")
}

// TestPublish_NoopTracer_DoesNotPanic verifica que tracer nil não causa panic.
func (s *PublisherSuite) TestPublish_NoopTracer_DoesNotPanic() {
	subs := []outbox.Subscription{s.sub1}
	names := []outbox.SubscriptionName{s.sub1.Name}

	s.registry.EXPECT().SubscriptionsFor(s.validType).Return(subs)
	s.storage.EXPECT().InsertEvent(mock.Anything, s.tx, mock.Anything).Return(nil)
	s.storage.EXPECT().InsertDeliveries(mock.Anything, s.tx, s.validID, names).Return(nil)

	s.NotPanics(func() {
		pub := outbox.NewPublisher(s.storage, s.registry, nil)
		_ = pub.Publish(s.ctx, s.tx, s.validEvt)
	})
}

func (s *PublisherSuite) TestPublish_WithMetrics_RecordsPublishedAfterPersistence() {
	obs := obsfake.NewProvider()
	metrics, err := outbox.NewOutboxMetrics(obs)
	s.Require().NoError(err)

	subs := []outbox.Subscription{s.sub1}
	names := []outbox.SubscriptionName{s.sub1.Name}

	s.registry.EXPECT().SubscriptionsFor(s.validType).Return(subs)
	s.storage.EXPECT().InsertEvent(mock.Anything, s.tx, mock.Anything).Return(nil)
	s.storage.EXPECT().InsertDeliveries(mock.Anything, s.tx, s.validID, names).Return(nil)

	pub := outbox.NewPublisherWithMetrics(s.storage, s.registry, nil, metrics)
	err = pub.Publish(s.ctx, s.tx, s.validEvt)
	s.NoError(err)

	fakeMetrics := obs.Metrics().(*obsfake.FakeMetrics)
	counter := fakeMetrics.GetCounter("outbox.events.published.total")
	s.Require().NotNil(counter)
	values := counter.GetValues()
	s.Require().Len(values, 1)
	s.Equal(int64(1), values[0].Value)
}
