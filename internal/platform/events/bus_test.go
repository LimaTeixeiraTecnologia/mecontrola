package events_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/clock"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
)

// testEvent é um evento mínimo para uso nos testes unitários.
type testEvent struct {
	name        events.EventName
	aggregateID string
	occurredAt  time.Time
}

func newTestEvent(s *suite.Suite, clk clock.Clock, aggregateID, action string) testEvent {
	s.T().Helper()
	n, err := events.NewEventName(fmt.Sprintf("test.%s", action))
	s.Require().NoError(err)
	return testEvent{
		name:        n,
		aggregateID: aggregateID,
		occurredAt:  clk.Now(),
	}
}

func (e testEvent) Name() events.EventName { return e.name }
func (e testEvent) OccurredAt() time.Time  { return e.occurredAt }
func (e testEvent) AggregateID() string    { return e.aggregateID }

// anotherEvent é um segundo tipo de evento para testar isolamento de tipo.
type anotherEvent struct {
	name        events.EventName
	aggregateID string
	occurredAt  time.Time
}

func (e anotherEvent) Name() events.EventName { return e.name }
func (e anotherEvent) OccurredAt() time.Time  { return e.occurredAt }
func (e anotherEvent) AggregateID() string    { return e.aggregateID }

// BusSuite testa o eventbus in-process tipado.
type BusSuite struct {
	suite.Suite
	ctx       context.Context
	fakeClock *clock.FakeClock
}

func TestBus(t *testing.T) {
	suite.Run(t, new(BusSuite))
}

func (s *BusSuite) SetupTest() {
	s.ctx = context.Background()
	s.fakeClock = clock.NewFakeClock(time.Time{})
}

func (s *BusSuite) TestPublishSubscribe() {
	s.Run("deve entregar eventos publicados a subscriber registrado", func() {
		bus := events.NewBus()

		var received []testEvent
		var mu sync.Mutex

		unsub, err := events.Subscribe(bus, func(_ context.Context, evt testEvent) error {
			mu.Lock()
			received = append(received, evt)
			mu.Unlock()
			return nil
		})
		s.Require().NoError(err)
		defer unsub()

		evt1 := newTestEvent(&s.Suite, s.fakeClock, "agg-1", "created")
		s.fakeClock.Advance(time.Millisecond)
		evt2 := newTestEvent(&s.Suite, s.fakeClock, "agg-2", "updated")

		s.Require().NoError(events.Publish(bus, s.ctx, evt1))
		s.Require().NoError(events.Publish(bus, s.ctx, evt2))

		shutdownCtx, cancel := context.WithTimeout(s.ctx, 2*time.Second)
		defer cancel()
		s.Require().NoError(bus.Close(shutdownCtx))

		mu.Lock()
		defer mu.Unlock()
		s.Len(received, 2)
		s.Equal("agg-1", received[0].AggregateID())
		s.Equal("agg-2", received[1].AggregateID())
	})
}

func (s *BusSuite) TestSubscribeUnsubscribe() {
	s.Run("deve parar de entregar eventos após unsubscribe", func() {
		bus := events.NewBus()
		var count atomic.Int64

		unsub, err := events.Subscribe(bus, func(_ context.Context, _ testEvent) error {
			count.Add(1)
			return nil
		})
		s.Require().NoError(err)

		evt := newTestEvent(&s.Suite, s.fakeClock, "agg-1", "created")
		s.Require().NoError(events.Publish(bus, s.ctx, evt))

		time.Sleep(10 * time.Millisecond)
		unsub()

		s.Require().NoError(events.Publish(bus, s.ctx, evt))

		shutdownCtx, cancel := context.WithTimeout(s.ctx, 2*time.Second)
		defer cancel()
		s.Require().NoError(bus.Close(shutdownCtx))

		s.Equal(int64(1), count.Load())
	})
}

func (s *BusSuite) TestMultipleSubscribersSameEventType() {
	s.Run("deve entregar evento a todos os subscribers do mesmo tipo", func() {
		bus := events.NewBus()
		var countA, countB atomic.Int64

		unsubA, err := events.Subscribe(bus, func(_ context.Context, _ testEvent) error {
			countA.Add(1)
			return nil
		})
		s.Require().NoError(err)
		defer unsubA()

		unsubB, err := events.Subscribe(bus, func(_ context.Context, _ testEvent) error {
			countB.Add(1)
			return nil
		})
		s.Require().NoError(err)
		defer unsubB()

		evt := newTestEvent(&s.Suite, s.fakeClock, "agg-1", "created")
		s.Require().NoError(events.Publish(bus, s.ctx, evt))

		shutdownCtx, cancel := context.WithTimeout(s.ctx, 2*time.Second)
		defer cancel()
		s.Require().NoError(bus.Close(shutdownCtx))

		s.Equal(int64(1), countA.Load())
		s.Equal(int64(1), countB.Load())
	})
}

func (s *BusSuite) TestTypeIsolation() {
	s.Run("deve isolar eventos por tipo e não entregar ao subscriber errado", func() {
		bus := events.NewBus()
		var countTest, countAnother atomic.Int64

		unsubTest, err := events.Subscribe(bus, func(_ context.Context, _ testEvent) error {
			countTest.Add(1)
			return nil
		})
		s.Require().NoError(err)
		defer unsubTest()

		nameAnother, err := events.NewEventName("test.another")
		s.Require().NoError(err)
		unsubAnother, err := events.Subscribe(bus, func(_ context.Context, _ anotherEvent) error {
			countAnother.Add(1)
			return nil
		})
		s.Require().NoError(err)
		defer unsubAnother()

		testEvt := newTestEvent(&s.Suite, s.fakeClock, "agg-1", "created")
		anotherEvt := anotherEvent{name: nameAnother, aggregateID: "agg-2", occurredAt: s.fakeClock.Now()}

		s.Require().NoError(events.Publish(bus, s.ctx, testEvt))
		s.Require().NoError(events.Publish(bus, s.ctx, anotherEvt))

		shutdownCtx, cancel := context.WithTimeout(s.ctx, 2*time.Second)
		defer cancel()
		s.Require().NoError(bus.Close(shutdownCtx))

		s.Equal(int64(1), countTest.Load(), "subscriber de testEvent deve receber apenas testEvent")
		s.Equal(int64(1), countAnother.Load(), "subscriber de anotherEvent deve receber apenas anotherEvent")
	})
}

func (s *BusSuite) TestCloseIdempotent() {
	s.Run("deve retornar nil ao fechar bus múltiplas vezes", func() {
		bus := events.NewBus()

		shutdownCtx, cancel := context.WithTimeout(s.ctx, 2*time.Second)
		defer cancel()

		s.Require().NoError(bus.Close(shutdownCtx))
		s.Require().NoError(bus.Close(shutdownCtx))
	})
}

func (s *BusSuite) TestPublishAfterClose() {
	s.Run("deve retornar ErrBusClosed ao publicar após close", func() {
		bus := events.NewBus()

		shutdownCtx, cancel := context.WithTimeout(s.ctx, 2*time.Second)
		defer cancel()
		s.Require().NoError(bus.Close(shutdownCtx))

		evt := newTestEvent(&s.Suite, s.fakeClock, "agg-1", "created")
		err := events.Publish(bus, s.ctx, evt)
		s.ErrorIs(err, events.ErrBusClosed)
	})
}

func (s *BusSuite) TestSubscribeAfterClose() {
	s.Run("deve retornar ErrBusClosed ao subscrever após close", func() {
		bus := events.NewBus()

		shutdownCtx, cancel := context.WithTimeout(s.ctx, 2*time.Second)
		defer cancel()
		s.Require().NoError(bus.Close(shutdownCtx))

		_, err := events.Subscribe(bus, func(_ context.Context, _ testEvent) error {
			return nil
		})
		s.ErrorIs(err, events.ErrBusClosed)
	})
}

func (s *BusSuite) TestBufferFullDoesNotBlock() {
	s.Run("deve descartar evento sem bloquear quando buffer estiver cheio", func() {
		bus := events.NewBus(events.WithBufferSize(1))

		releaser := make(chan struct{})
		subscribed := make(chan struct{}, 1)

		unsub, err := events.Subscribe(bus, func(_ context.Context, _ testEvent) error {
			select {
			case subscribed <- struct{}{}:
			default:
			}
			select {
			case <-releaser:
			case <-time.After(3 * time.Second):
			}
			return nil
		})
		s.Require().NoError(err)

		evt0 := newTestEvent(&s.Suite, s.fakeClock, "agg-0", "start")
		s.Require().NoError(events.Publish(bus, s.ctx, evt0))

		select {
		case <-subscribed:
		case <-time.After(2 * time.Second):
			close(releaser)
			s.Fail("handler não iniciou a tempo")
		}

		evt1 := newTestEvent(&s.Suite, s.fakeClock, "agg-1", "fill")
		s.Require().NoError(events.Publish(bus, s.ctx, evt1))

		publishDone := make(chan error, 1)
		go func() {
			evt2 := newTestEvent(&s.Suite, s.fakeClock, "agg-2", "drop")
			publishDone <- events.Publish(bus, s.ctx, evt2)
		}()

		select {
		case pubErr := <-publishDone:
			s.NoError(pubErr, "Publish com buffer cheio deve retornar nil (drop silencioso)")
		case <-time.After(500 * time.Millisecond):
			close(releaser)
			unsub()
			s.Fail("Publish bloqueou com buffer cheio — violação de backpressure por drop")
		}

		close(releaser)
		unsub()

		shutdownCtx, cancel := context.WithTimeout(s.ctx, 3*time.Second)
		defer cancel()
		s.Require().NoError(bus.Close(shutdownCtx))
	})
}

func (s *BusSuite) TestCloseDrainBeforeTimeout() {
	s.Run("deve processar todos os eventos do buffer antes de completar close", func() {
		bus := events.NewBus(events.WithBufferSize(10))
		var processed atomic.Int64
		const total = 5

		unsub, err := events.Subscribe(bus, func(_ context.Context, _ testEvent) error {
			processed.Add(1)
			return nil
		})
		s.Require().NoError(err)
		defer unsub()

		for i := range total {
			evt := newTestEvent(&s.Suite, s.fakeClock, fmt.Sprintf("agg-%d", i), "ping")
			s.Require().NoError(events.Publish(bus, s.ctx, evt))
		}

		shutdownCtx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
		defer cancel()
		s.Require().NoError(bus.Close(shutdownCtx))

		s.Equal(int64(total), processed.Load())
	})
}
