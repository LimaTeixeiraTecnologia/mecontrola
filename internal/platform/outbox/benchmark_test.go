package outbox_test

import (
	"context"
	"encoding/json"
	"math/rand"
	"testing"
	"time"

	testifymock "github.com/stretchr/testify/mock"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox/mocks"
)

// buildBenchmarkEvent cria um Event válido para uso em benchmarks.
func buildBenchmarkEvent(b *testing.B) outbox.Event {
	b.Helper()
	evtID, err := events.NewEventID("01JTEST000000000000000001")
	if err != nil {
		b.Fatal(err)
	}
	evtType, err := events.NewEventName("order.placed")
	if err != nil {
		b.Fatal(err)
	}
	evt, err := outbox.NewEvent(outbox.NewEventParams{
		ID:            evtID,
		EventType:     evtType,
		AggregateType: "order",
		AggregateID:   "bench-123",
		Payload:       json.RawMessage(`{"id":"bench-123","amount":100}`),
	})
	if err != nil {
		b.Fatal(err)
	}
	return evt
}

// buildBenchmarkPublisher cria um Publisher com mock-fast de Storage e Registry para benchmarks.
// O mock Storage retorna nil para InsertEvent e InsertDeliveries (sem I/O real).
func buildBenchmarkPublisher(b *testing.B, numHandlers int) (outbox.Publisher, *stubDBTX) {
	b.Helper()

	storage := mocks.NewStorage(b)
	registry := mocks.NewRegistry(b)

	evtType, err := events.NewEventName("order.placed")
	if err != nil {
		b.Fatal(err)
	}

	subs := make([]outbox.Subscription, numHandlers)
	for i := range numHandlers {
		name, nerr := outbox.NewSubscriptionName("bench-handler-" + string(rune('a'+i)))
		if nerr != nil {
			b.Fatal(nerr)
		}
		subs[i] = outbox.Subscription{
			Name:      name,
			EventType: evtType,
			Handler:   func(_ context.Context, _ outbox.Event) error { return nil },
		}
	}

	names := make([]outbox.SubscriptionName, numHandlers)
	for i, s := range subs {
		names[i] = s.Name
	}

	registry.On("SubscriptionsFor", evtType).Return(subs)
	storage.On("InsertEvent", testifymock.Anything, testifymock.Anything, testifymock.Anything).Return(nil)
	storage.On("InsertDeliveries", testifymock.Anything, testifymock.Anything, testifymock.Anything, testifymock.Anything).Return(nil)

	pub := outbox.NewPublisher(storage, registry, nil)
	tx := &stubDBTX{}
	return pub, tx
}

// BenchmarkPublisher_Publish_1Handler mede throughput de Publish com 1 handler registrado.
// Storage é mock-fast (sem I/O real) — mede overhead puro do Publisher.
func BenchmarkPublisher_Publish_1Handler(b *testing.B) {
	pub, tx := buildBenchmarkPublisher(b, 1)
	evt := buildBenchmarkEvent(b)
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		if err := pub.Publish(ctx, tx, evt); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPublisher_Publish_3Handlers mede throughput de Publish com 3 handlers registrados.
func BenchmarkPublisher_Publish_3Handlers(b *testing.B) {
	pub, tx := buildBenchmarkPublisher(b, 3)
	evt := buildBenchmarkEvent(b)
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		if err := pub.Publish(ctx, tx, evt); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPublisher_Publish_5Handlers mede throughput de Publish com 5 handlers registrados.
func BenchmarkPublisher_Publish_5Handlers(b *testing.B) {
	pub, tx := buildBenchmarkPublisher(b, 5)
	evt := buildBenchmarkEvent(b)
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		if err := pub.Publish(ctx, tx, evt); err != nil {
			b.Fatal(err)
		}
	}
}

// drainBenchFixtures agrupa os fixtures reutilizáveis do BenchmarkDispatcher_DrainBacklog.
type drainBenchFixtures struct {
	claims   []outbox.Claim
	policy   outbox.BackoffPolicy
	registry outbox.Registry
}

// buildDrainBenchFixtures cria fixtures de benchmark para o Dispatcher sem I/O real.
func buildDrainBenchFixtures(b *testing.B, backlogSize int) drainBenchFixtures {
	b.Helper()

	evtID, err := events.NewEventID("01JTEST000000000000000001")
	if err != nil {
		b.Fatal(err)
	}
	evtType, err := events.NewEventName("order.placed")
	if err != nil {
		b.Fatal(err)
	}
	subName, err := outbox.NewSubscriptionName("bench-drain-handler")
	if err != nil {
		b.Fatal(err)
	}
	evt, err := outbox.NewEvent(outbox.NewEventParams{
		ID:            evtID,
		EventType:     evtType,
		AggregateType: "order",
		AggregateID:   "drain-bench",
		Payload:       json.RawMessage(`{"id":"drain-bench"}`),
	})
	if err != nil {
		b.Fatal(err)
	}

	claims := make([]outbox.Claim, backlogSize)
	for i := range claims {
		claims[i] = outbox.Claim{
			ID:               outbox.ClaimID(int64(i + 1)),
			Event:            evt,
			SubscriptionName: subName,
			Attempt:          outbox.NewAttempt(1),
			ClaimedAt:        time.Now().UTC(),
		}
	}

	rng := rand.New(rand.NewSource(42)) //nolint:gosec
	policy, err := outbox.NewBackoffPolicy(time.Second, 5*time.Minute, rng)
	if err != nil {
		b.Fatal(err)
	}

	reg := outbox.NewRegistry()
	_ = reg.Register(outbox.Subscription{
		Name:      subName,
		EventType: evtType,
		Handler:   func(_ context.Context, _ outbox.Event) error { return nil },
	})

	return drainBenchFixtures{claims: claims, policy: policy, registry: reg}
}

// BenchmarkDispatcher_DrainBacklog mede throughput do Dispatcher drenando um backlog pré-populado
// com handler dummy de latência zero (mock-fast). Avalia throughput sustentado do loop de dispatch.
//
// Nota: este benchmark usa mocks (sem Postgres real). Para o benchmark com testcontainer real,
// ver BenchmarkPublisher_Publish_Postgres e BenchmarkDispatcher_DrainBacklog com build tag integration.
func BenchmarkDispatcher_DrainBacklog(b *testing.B) {
	const backlogSize = 1000

	fix := buildDrainBenchFixtures(b, backlogSize)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		runDrainIteration(b, fix, backlogSize)
	}

	// Reporta throughput em deliveries/s.
	b.ReportMetric(float64(backlogSize*b.N)/b.Elapsed().Seconds(), "deliveries/s")
}

// runDrainIteration executa uma iteração do drain benchmark isolada para controle de função-length.
func runDrainIteration(b *testing.B, fix drainBenchFixtures, backlogSize int) {
	b.Helper()

	storage := mocks.NewStorage(b)
	// Primeira chamada retorna o backlog completo; chamadas subsequentes retornam vazio.
	storage.On("ClaimReady", testifymock.Anything, testifymock.Anything, testifymock.Anything).
		Return(fix.claims, nil).Once()
	storage.On("ClaimReady", testifymock.Anything, testifymock.Anything, testifymock.Anything).
		Return([]outbox.Claim{}, nil)
	storage.On("MarkProcessed", testifymock.Anything, testifymock.Anything, testifymock.Anything).
		Return(nil)

	dispatcher, derr := outbox.NewDispatcher(outbox.DispatcherConfig{
		Enabled:        true,
		Storage:        storage,
		Registry:       fix.registry,
		Policy:         fix.policy,
		MaxAttempts:    outbox.NewAttempt(15),
		HandlerTimeout: 30 * time.Second,
		TickInterval:   time.Millisecond,
		BatchSize:      backlogSize,
		InstanceID:     "bench-instance",
	})
	if derr != nil {
		b.Fatal(derr)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_ = dispatcher.Start(ctx)
	// Aguarda um tick para processar o backlog.
	time.Sleep(10 * time.Millisecond)
	_ = dispatcher.Stop(ctx)
}
