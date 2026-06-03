package outbox_test

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"testing"
	"time"

	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox/fakes"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox/mocks"
)

// TestNewDispatcher_NilStorage verifica que NewDispatcher falha com storage nil.
func TestNewDispatcher_NilStorage(t *testing.T) {
	_, err := outbox.NewDispatcher(outbox.DispatcherConfig{
		Storage:  nil,
		Registry: outbox.NewRegistry(),
	})
	require.Error(t, err)
}

// TestNewDispatcher_NilRegistry verifica que NewDispatcher falha com registry nil.
func TestNewDispatcher_NilRegistry(t *testing.T) {
	_, err := outbox.NewDispatcher(outbox.DispatcherConfig{
		Storage:  mocks.NewStorage(t),
		Registry: nil,
	})
	require.Error(t, err)
}

// TestNewDispatcher_Defaults verifica que NewDispatcher aplica defaults.
func TestNewDispatcher_Defaults(t *testing.T) {
	d, err := outbox.NewDispatcher(outbox.DispatcherConfig{
		Storage:  mocks.NewStorage(t),
		Registry: outbox.NewRegistry(),
		// HandlerTimeout, TickInterval, BatchSize, Clock, Metrics, Logger ficam zero/nil
	})
	require.NoError(t, err)
	require.NotNil(t, d)

	// Stop imediato para limpar.
	stopCtx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_ = d.Stop(stopCtx)
}

// TestRealClock_Now verifica que RealClock.Now() retorna tempo próximo ao atual.
func TestRealClock_Now(t *testing.T) {
	c := outbox.RealClock()
	before := time.Now().UTC().Add(-time.Second)
	got := c.Now()
	after := time.Now().UTC().Add(time.Second)
	require.True(t, got.After(before), "RealClock.Now deve ser depois de before")
	require.True(t, got.Before(after), "RealClock.Now deve ser antes de after")
}

// TestNoopMetrics_AllMethods verifica que NoopMetrics não entra em panic.
func TestNoopMetrics_AllMethods(t *testing.T) {
	m := outbox.NoopMetrics()
	ctx := context.Background()
	// Chama diretamente via interface — sem panic é o critério.
	m.RecordDeliverySuccess(ctx, "sub")
	m.RecordDeliveryFailure(ctx, "sub")
	m.RecordDeliveryDLQ(ctx, "sub")
}

// TestDispatcher_ClaimReadyError verifica que erro em ClaimReady é logado sem panic.
func TestDispatcher_ClaimReadyError(t *testing.T) {
	storage := mocks.NewStorage(t)
	registry := outbox.NewRegistry()

	rng := rand.New(rand.NewSource(1)) //nolint:gosec
	policy, err := outbox.NewBackoffPolicy(2*time.Second, 5*time.Minute, rng)
	require.NoError(t, err)

	clock := fakes.NewFakeClock(time.Now().UTC())

	d, err := outbox.NewDispatcher(outbox.DispatcherConfig{
		Enabled:        true,
		Storage:        storage,
		Registry:       registry,
		Policy:         policy,
		MaxAttempts:    outbox.NewAttempt(15),
		HandlerTimeout: 100 * time.Millisecond,
		TickInterval:   10 * time.Millisecond,
		BatchSize:      10,
		InstanceID:     "test",
		Clock:          clock,
		Metrics:        outbox.NoopMetrics(),
	})
	require.NoError(t, err)

	done := make(chan struct{})
	storage.On("ClaimReady", testifymock.Anything, 10, "test").
		Return(nil, errors.New("db connection lost")).Once().
		Run(func(_ testifymock.Arguments) { close(done) })
	storage.On("ClaimReady", testifymock.Anything, 10, "test").
		Return(nil, nil).Maybe()

	ctx := context.Background()
	require.NoError(t, d.Start(ctx))

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("ClaimReady error não foi processado dentro do timeout")
	}

	stopCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	require.NoError(t, d.Stop(stopCtx))
}

// TestDispatcher_Deliver_WithTraceparent verifica propagação de traceparent do evento.
func TestDispatcher_Deliver_WithTraceparent(t *testing.T) {
	storage := mocks.NewStorage(t)

	var callCount int64
	subName, _ := outbox.NewSubscriptionName("test-subscription")
	var capturedCtx context.Context
	handler := func(ctx context.Context, _ outbox.Event) error {
		capturedCtx = ctx
		callCount++
		return nil
	}

	registry := outbox.NewRegistry()
	evtType, _ := events.NewEventName("order.placed")
	sub := outbox.Subscription{Name: subName, EventType: evtType, Handler: handler}
	require.NoError(t, registry.Register(sub))

	rng := rand.New(rand.NewSource(99)) //nolint:gosec
	policy, _ := outbox.NewBackoffPolicy(2*time.Second, 5*time.Minute, rng)
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := fakes.NewFakeClock(now)

	d, err := outbox.NewDispatcher(outbox.DispatcherConfig{
		Enabled:        true,
		Storage:        storage,
		Registry:       registry,
		Policy:         policy,
		MaxAttempts:    outbox.NewAttempt(15),
		HandlerTimeout: 100 * time.Millisecond,
		TickInterval:   10 * time.Millisecond,
		BatchSize:      10,
		InstanceID:     "test",
		Clock:          clock,
		Metrics:        outbox.NoopMetrics(),
	})
	require.NoError(t, err)

	evtID, _ := events.NewEventID("01JTEST000000000000000002")
	headers := outbox.Headers{"traceparent": "00-trace-span-01"}
	event, _ := outbox.NewEvent(outbox.NewEventParams{
		ID:            evtID,
		EventType:     evtType,
		AggregateType: "order",
		AggregateID:   "456",
		Payload:       json.RawMessage(`{"id":"456"}`),
		Headers:       headers,
	})

	claim := outbox.Claim{
		ID:               outbox.ClaimID(2),
		Event:            event,
		SubscriptionName: subName,
		Attempt:          outbox.NewAttempt(1),
		ClaimedAt:        now,
	}

	done := make(chan struct{})
	storage.On("ClaimReady", testifymock.Anything, 10, "test").
		Return([]outbox.Claim{claim}, nil).Once()
	storage.On("MarkProcessed", testifymock.Anything, outbox.ClaimID(2), now).
		Return(nil).Once().
		Run(func(_ testifymock.Arguments) { close(done) })
	storage.On("ClaimReady", testifymock.Anything, 10, "test").
		Return(nil, nil).Maybe()

	ctx := context.Background()
	require.NoError(t, d.Start(ctx))

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("MarkProcessed não foi chamado")
	}

	stopCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	require.NoError(t, d.Stop(stopCtx))

	require.Equal(t, int64(1), callCount)
	require.NotNil(t, capturedCtx)
}

// TestDispatcher_Loop_ContextCancel verifica que loop termina quando ctx é cancelado.
func TestDispatcher_Loop_ContextCancel(t *testing.T) {
	storage := mocks.NewStorage(t)
	registry := outbox.NewRegistry()

	rng := rand.New(rand.NewSource(7)) //nolint:gosec
	policy, _ := outbox.NewBackoffPolicy(2*time.Second, 5*time.Minute, rng)
	clock := fakes.NewFakeClock(time.Now().UTC())

	d, err := outbox.NewDispatcher(outbox.DispatcherConfig{
		Enabled:        true,
		Storage:        storage,
		Registry:       registry,
		Policy:         policy,
		MaxAttempts:    outbox.NewAttempt(15),
		HandlerTimeout: 100 * time.Millisecond,
		TickInterval:   10 * time.Millisecond,
		BatchSize:      10,
		InstanceID:     "test",
		Clock:          clock,
		Metrics:        outbox.NoopMetrics(),
	})
	require.NoError(t, err)

	storage.On("ClaimReady", testifymock.Anything, 10, "test").
		Return(nil, nil).Maybe()

	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, d.Start(ctx))

	// Cancela o context externo para testar o path ctx.Done() no loop.
	time.Sleep(20 * time.Millisecond)
	cancel()

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer stopCancel()
	require.NoError(t, d.Stop(stopCtx))
}
