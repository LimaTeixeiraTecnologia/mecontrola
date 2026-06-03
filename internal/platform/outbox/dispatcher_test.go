package outbox_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math/rand"
	"sync/atomic"
	"testing"
	"time"

	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox/fakes"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox/mocks"
)

// DispatcherSuite cobre os 8 cenários obrigatórios do Dispatcher (RF-33).
type DispatcherSuite struct {
	suite.Suite
	now     time.Time
	clock   *fakes.FakeClock
	storage *mocks.Storage
	policy  outbox.BackoffPolicy
	event   outbox.Event
	subName outbox.SubscriptionName
}

func TestDispatcherSuite(t *testing.T) {
	suite.Run(t, new(DispatcherSuite))
}

func (s *DispatcherSuite) SetupTest() {
	s.now = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	s.clock = fakes.NewFakeClock(s.now)
	s.storage = mocks.NewStorage(s.T())

	// BackoffPolicy determinístico para testes.
	rng := rand.New(rand.NewSource(42)) //nolint:gosec
	var err error
	s.policy, err = outbox.NewBackoffPolicy(2*time.Second, 5*time.Minute, rng)
	s.Require().NoError(err)

	// SubscriptionName válida.
	s.subName, err = outbox.NewSubscriptionName("test-subscription")
	s.Require().NoError(err)

	// Evento de teste.
	evtID, err := events.NewEventID("01JTEST000000000000000001")
	s.Require().NoError(err)

	evtType, err := events.NewEventName("order.placed")
	s.Require().NoError(err)

	s.event, err = outbox.NewEvent(outbox.NewEventParams{
		ID:            evtID,
		EventType:     evtType,
		AggregateType: "order",
		AggregateID:   "123",
		Payload:       json.RawMessage(`{"id":"123"}`),
	})
	s.Require().NoError(err)
}

// buildDispatcher cria um Dispatcher com configuração padrão de teste.
func (s *DispatcherSuite) buildDispatcher(enabled bool, handler outbox.Handler, maxAttempts uint8) *outbox.Dispatcher {
	registry := outbox.NewRegistry()
	if handler != nil {
		evtType, _ := events.NewEventName("order.placed")
		sub := outbox.Subscription{
			Name:      s.subName,
			EventType: evtType,
			Handler:   handler,
		}
		s.Require().NoError(registry.Register(sub))
	}

	d, err := outbox.NewDispatcher(outbox.DispatcherConfig{
		Enabled:        enabled,
		Storage:        s.storage,
		Registry:       registry,
		Policy:         s.policy,
		MaxAttempts:    outbox.NewAttempt(maxAttempts),
		HandlerTimeout: 100 * time.Millisecond,
		TickInterval:   10 * time.Millisecond,
		BatchSize:      10,
		InstanceID:     "test-instance",
		Clock:          s.clock,
		Metrics:        outbox.NoopMetrics(),
	})
	s.Require().NoError(err)
	return d
}

// buildClaim cria um Claim de teste com os parâmetros informados.
func (s *DispatcherSuite) buildClaim(attempt uint8) outbox.Claim {
	return outbox.Claim{
		ID:               outbox.ClaimID(1),
		Event:            s.event,
		SubscriptionName: s.subName,
		Attempt:          outbox.NewAttempt(attempt),
		ClaimedAt:        s.now,
	}
}

// Cenário 1: Sucesso → MarkProcessed chamado com processedAt definido.
func (s *DispatcherSuite) TestScenario1_Success_CallsMarkProcessed() {
	var callCount int64
	handler := fakes.SuccessHandler(&callCount)
	d := s.buildDispatcher(true, handler, 15)

	claim := s.buildClaim(1)
	done := make(chan struct{})

	s.storage.On("ClaimReady", testifymock.Anything, 10, "test-instance").
		Return([]outbox.Claim{claim}, nil).Once()
	s.storage.On("MarkProcessed", testifymock.Anything, outbox.ClaimID(1), s.now).
		Return(nil).Once().
		Run(func(_ testifymock.Arguments) { close(done) })

	// Segunda chamada retorna vazio para parar loop.
	s.storage.On("ClaimReady", testifymock.Anything, 10, "test-instance").
		Return(nil, nil).Maybe()

	ctx := context.Background()
	s.Require().NoError(d.Start(ctx))

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		s.Fail("MarkProcessed não foi chamado dentro do timeout")
	}

	stopCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	s.Require().NoError(d.Stop(stopCtx))

	s.Equal(int64(1), atomic.LoadInt64(&callCount))
}

// Cenário 2: Erro transitório → MarkFailed com nextRetryAt > now preservando attempt do claim.
func (s *DispatcherSuite) TestScenario2_TransientError_CallsMarkFailed() {
	var callCount int64
	handler := fakes.TransientHandler(&callCount, "connection reset")
	d := s.buildDispatcher(true, handler, 15)

	claim := s.buildClaim(1) // attempt=1
	done := make(chan struct{})

	s.storage.On("ClaimReady", testifymock.Anything, 10, "test-instance").
		Return([]outbox.Claim{claim}, nil).Once()
	s.storage.On("MarkFailed", testifymock.Anything, outbox.ClaimID(1), testifymock.AnythingOfType("string"),
		outbox.NewAttempt(1), testifymock.MatchedBy(func(t time.Time) bool { return t.After(s.now) })).
		Return(nil).Once().
		Run(func(_ testifymock.Arguments) { close(done) })

	s.storage.On("ClaimReady", testifymock.Anything, 10, "test-instance").
		Return(nil, nil).Maybe()

	ctx := context.Background()
	s.Require().NoError(d.Start(ctx))

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		s.Fail("MarkFailed não foi chamado dentro do timeout")
	}

	stopCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	s.Require().NoError(d.Stop(stopCtx))

	s.Equal(int64(1), atomic.LoadInt64(&callCount))
}

// Cenário 3: ErrPermanent → MarkDLQ imediato sem incrementar attempts.
func (s *DispatcherSuite) TestScenario3_ErrPermanent_CallsMarkDLQImmediately() {
	var callCount int64
	handler := fakes.PermanentHandler(&callCount, "schema incompatível")
	d := s.buildDispatcher(true, handler, 15)

	claim := s.buildClaim(1) // attempt=1 (não deve ser incrementado via MarkFailed)
	done := make(chan struct{})

	s.storage.On("ClaimReady", testifymock.Anything, 10, "test-instance").
		Return([]outbox.Claim{claim}, nil).Once()
	s.storage.On("MarkDLQ", testifymock.Anything, outbox.ClaimID(1),
		testifymock.AnythingOfType("string"), s.now).
		Return(nil).Once().
		Run(func(_ testifymock.Arguments) { close(done) })

	s.storage.On("ClaimReady", testifymock.Anything, 10, "test-instance").
		Return(nil, nil).Maybe()

	ctx := context.Background()
	s.Require().NoError(d.Start(ctx))

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		s.Fail("MarkDLQ não foi chamado dentro do timeout")
	}

	stopCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	s.Require().NoError(d.Stop(stopCtx))

	s.Equal(int64(1), atomic.LoadInt64(&callCount))
	// MarkFailed NÃO deve ter sido chamado.
	s.storage.AssertNumberOfCalls(s.T(), "MarkFailed", 0)
}

// Cenário 4: Exhaustão (attempt >= maxAttempts) com erro transitório → MarkDLQ.
func (s *DispatcherSuite) TestScenario4_ExhaustedAttempts_CallsMarkDLQ() {
	var callCount int64
	handler := fakes.TransientHandler(&callCount, "timeout de banco")
	d := s.buildDispatcher(true, handler, 15)

	// attempt=15 que é >= maxAttempts(15) → IsExhausted = true → DLQ.
	claim := s.buildClaim(15)
	done := make(chan struct{})

	s.storage.On("ClaimReady", testifymock.Anything, 10, "test-instance").
		Return([]outbox.Claim{claim}, nil).Once()
	s.storage.On("MarkDLQ", testifymock.Anything, outbox.ClaimID(1), "timeout de banco", s.now).
		Return(nil).Once().
		Run(func(_ testifymock.Arguments) { close(done) })

	s.storage.On("ClaimReady", testifymock.Anything, 10, "test-instance").
		Return(nil, nil).Maybe()

	ctx := context.Background()
	s.Require().NoError(d.Start(ctx))

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		s.Fail("MarkDLQ não foi chamado dentro do timeout para exhaustão")
	}

	stopCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	s.Require().NoError(d.Stop(stopCtx))
}

// Cenário 5: Timeout do handler → MarkFailed (tratado como transitório).
func (s *DispatcherSuite) TestScenario5_HandlerTimeout_CallsMarkFailed() {
	var startCount, doneCount int64
	handler := fakes.TimeoutHandler(&startCount, &doneCount)

	registry := outbox.NewRegistry()
	evtType, _ := events.NewEventName("order.placed")
	sub := outbox.Subscription{Name: s.subName, EventType: evtType, Handler: handler}
	s.Require().NoError(registry.Register(sub))

	d, err := outbox.NewDispatcher(outbox.DispatcherConfig{
		Enabled:        true,
		Storage:        s.storage,
		Registry:       registry,
		Policy:         s.policy,
		MaxAttempts:    outbox.NewAttempt(15),
		HandlerTimeout: 50 * time.Millisecond, // timeout muito curto para forçar expiração
		TickInterval:   10 * time.Millisecond,
		BatchSize:      10,
		InstanceID:     "test-instance",
		Clock:          s.clock,
		Metrics:        outbox.NoopMetrics(),
	})
	s.Require().NoError(err)

	claim := s.buildClaim(1)
	done := make(chan struct{})

	s.storage.On("ClaimReady", testifymock.Anything, 10, "test-instance").
		Return([]outbox.Claim{claim}, nil).Once()
	s.storage.On("MarkFailed", testifymock.Anything, outbox.ClaimID(1),
		testifymock.AnythingOfType("string"),
		outbox.NewAttempt(1),
		testifymock.MatchedBy(func(t time.Time) bool { return t.After(s.now) })).
		Return(nil).Once().
		Run(func(_ testifymock.Arguments) { close(done) })

	s.storage.On("ClaimReady", testifymock.Anything, 10, "test-instance").
		Return(nil, nil).Maybe()

	ctx := context.Background()
	s.Require().NoError(d.Start(ctx))

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		s.Fail("MarkFailed não foi chamado para timeout do handler")
	}

	stopCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	s.Require().NoError(d.Stop(stopCtx))

	s.Equal(int64(1), atomic.LoadInt64(&startCount))
}

// Cenário 6: Panic do handler → MarkDLQ (classificado como permanent via recover).
func (s *DispatcherSuite) TestScenario6_HandlerPanic_CallsMarkDLQ() {
	var callCount int64
	handler := fakes.PanicHandler(&callCount, "unexpected nil pointer")
	d := s.buildDispatcher(true, handler, 15)

	claim := s.buildClaim(1)
	done := make(chan struct{})

	s.storage.On("ClaimReady", testifymock.Anything, 10, "test-instance").
		Return([]outbox.Claim{claim}, nil).Once()
	s.storage.On("MarkDLQ", testifymock.Anything, outbox.ClaimID(1),
		testifymock.AnythingOfType("string"), s.now).
		Return(nil).Once().
		Run(func(_ testifymock.Arguments) { close(done) })

	s.storage.On("ClaimReady", testifymock.Anything, 10, "test-instance").
		Return(nil, nil).Maybe()

	ctx := context.Background()
	s.Require().NoError(d.Start(ctx))

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		s.Fail("MarkDLQ não foi chamado após panic do handler")
	}

	stopCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	s.Require().NoError(d.Stop(stopCtx))

	s.Equal(int64(1), atomic.LoadInt64(&callCount))
}

// Cenário 7: OUTBOX_DISPATCHER_ENABLED=false → goroutine não inicia, ticker não cria,
// Storage.ClaimReady nunca chamado.
func (s *DispatcherSuite) TestScenario7_DispatcherDisabled_NeverCallsClaimReady() {
	d := s.buildDispatcher(false, nil, 15)

	ctx := context.Background()
	s.Require().NoError(d.Start(ctx))

	// Aguarda um intervalo para garantir que nenhuma goroutine disparou.
	time.Sleep(50 * time.Millisecond)

	stopCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	s.Require().NoError(d.Stop(stopCtx))

	s.storage.AssertNumberOfCalls(s.T(), "ClaimReady", 0)
}

func (s *DispatcherSuite) TestScenario7_DispatcherDisabled_LogsRunbookEventName() {
	var logBuffer bytes.Buffer
	d, err := outbox.NewDispatcher(outbox.DispatcherConfig{
		Enabled:        false,
		Storage:        s.storage,
		Registry:       outbox.NewRegistry(),
		Policy:         s.policy,
		MaxAttempts:    outbox.NewAttempt(15),
		HandlerTimeout: 100 * time.Millisecond,
		TickInterval:   10 * time.Millisecond,
		BatchSize:      10,
		InstanceID:     "test-instance",
		Clock:          s.clock,
		Logger:         slog.New(slog.NewTextHandler(&logBuffer, nil)),
	})
	s.Require().NoError(err)
	s.Require().NoError(d.Start(context.Background()))
	s.Contains(logBuffer.String(), "outbox.dispatcher.disabled")
}

func (s *DispatcherSuite) TestStart_LogsRunbookStartedEventName() {
	var logBuffer bytes.Buffer
	d, err := outbox.NewDispatcher(outbox.DispatcherConfig{
		Enabled:        true,
		Storage:        s.storage,
		Registry:       outbox.NewRegistry(),
		Policy:         s.policy,
		MaxAttempts:    outbox.NewAttempt(15),
		HandlerTimeout: 100 * time.Millisecond,
		TickInterval:   10 * time.Second,
		BatchSize:      10,
		InstanceID:     "test-instance",
		Clock:          s.clock,
		Logger:         slog.New(slog.NewTextHandler(&logBuffer, nil)),
	})
	s.Require().NoError(err)

	ctx := context.Background()
	s.Require().NoError(d.Start(ctx))
	s.Contains(logBuffer.String(), "outbox.dispatcher.started")

	stopCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	s.Require().NoError(d.Stop(stopCtx))
}

// Cenário 8: Stop com handler in-flight: handler termina antes do Stop retornar.
func (s *DispatcherSuite) TestScenario8_StopWaitsForInFlightHandlers() {
	release := make(chan struct{})
	var startCount int64
	handler := fakes.BlockingHandler(&startCount, release)
	d := s.buildDispatcher(true, handler, 15)

	claim := s.buildClaim(1)

	s.storage.On("ClaimReady", testifymock.Anything, 10, "test-instance").
		Return([]outbox.Claim{claim}, nil).Once()
	s.storage.On("MarkProcessed", testifymock.Anything, outbox.ClaimID(1), s.now).
		Return(nil).Once()
	s.storage.On("ClaimReady", testifymock.Anything, 10, "test-instance").
		Return(nil, nil).Maybe()

	ctx := context.Background()
	s.Require().NoError(d.Start(ctx))

	// Espera o handler iniciar.
	s.Eventually(func() bool {
		return atomic.LoadInt64(&startCount) > 0
	}, 2*time.Second, 5*time.Millisecond, "handler in-flight não iniciou dentro do timeout")

	// Stop com timeout generoso; libera o handler após 20ms.
	stopCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	stopDone := make(chan error, 1)
	stopStart := time.Now()
	go func() {
		stopDone <- d.Stop(stopCtx)
	}()

	// Libera o handler in-flight após 20ms.
	time.Sleep(20 * time.Millisecond)
	close(release)

	select {
	case err := <-stopDone:
		elapsed := time.Since(stopStart)
		s.Require().NoError(err)
		s.Less(elapsed, 2*time.Second, "Stop demorou mais do que o timeout esperado")
	case <-time.After(3 * time.Second):
		s.Fail("Stop não retornou dentro do timeout")
	}

	s.Equal(int64(1), atomic.LoadInt64(&startCount))
}

// TestDispatcher_Stop_Idempotent verifica que Stop pode ser chamado múltiplas vezes sem panic.
func (s *DispatcherSuite) TestDispatcher_Stop_Idempotent() {
	d := s.buildDispatcher(false, nil, 15)

	ctx := context.Background()
	s.Require().NoError(d.Start(ctx))

	stopCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	s.Require().NoError(d.Stop(stopCtx))

	stopCtx2, cancel2 := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel2()
	s.Require().NoError(d.Stop(stopCtx2))
}

// TestDispatcher_StopTimeout verifica que Stop retorna ctx.Err() quando o handler
// não termina antes do ctx.
func (s *DispatcherSuite) TestDispatcher_StopTimeout() {
	release := make(chan struct{})
	var startCount int64
	handler := fakes.BlockingHandler(&startCount, release)
	d := s.buildDispatcher(true, handler, 15)

	claim := s.buildClaim(1)

	s.storage.On("ClaimReady", testifymock.Anything, 10, "test-instance").
		Return([]outbox.Claim{claim}, nil).Once()
	s.storage.On("ClaimReady", testifymock.Anything, 10, "test-instance").
		Return(nil, nil).Maybe()
	// O handler vai terminar com sucesso depois que release for fechado.
	s.storage.On("MarkProcessed", testifymock.Anything, outbox.ClaimID(1), s.now).
		Return(nil).Maybe()

	ctx := context.Background()
	s.Require().NoError(d.Start(ctx))

	s.Eventually(func() bool {
		return atomic.LoadInt64(&startCount) > 0
	}, 2*time.Second, 5*time.Millisecond, "handler in-flight não iniciou")

	// Stop com timeout muito curto — deve retornar erro de timeout.
	stopCtx, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
	defer cancel()
	err := d.Stop(stopCtx)
	s.Require().Error(err)
	s.True(errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled))

	// Libera o handler para não vazar goroutine.
	close(release)
	// Aguarda o handler terminar para evitar goroutine leak.
	time.Sleep(50 * time.Millisecond)
}
