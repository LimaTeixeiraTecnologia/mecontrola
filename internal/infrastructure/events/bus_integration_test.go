//go:build integration

package events_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/clock"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/events"
)

// BusIntegrationSuite valida o Bus sob carga concorrente e cenários de lifecycle.
type BusIntegrationSuite struct {
	suite.Suite
	ctx       context.Context
	fakeClock *clock.FakeClock
}

func TestBusIntegration(t *testing.T) {
	suite.Run(t, new(BusIntegrationSuite))
}

func (s *BusIntegrationSuite) SetupTest() {
	s.ctx = context.Background()
	s.fakeClock = clock.NewFakeClock(time.Time{})
}

// TestConcurrentEvents valida o Bus sob carga de 1000 eventos concorrentes.
func (s *BusIntegrationSuite) TestConcurrentEvents() {
	s.Run("deve receber 1000 eventos com 2 subscribers sem data race", func() {
		const total = 1000
		const numPublishers = 10

		bus := events.NewBus(events.WithBufferSize(total + 100))

		var receivedA []string
		var receivedB []string
		var muA, muB sync.Mutex

		unsubA, err := events.Subscribe(bus, func(_ context.Context, evt testEvent) error {
			muA.Lock()
			receivedA = append(receivedA, evt.AggregateID())
			muA.Unlock()
			return nil
		})
		s.Require().NoError(err)
		defer unsubA()

		unsubB, err := events.Subscribe(bus, func(_ context.Context, evt testEvent) error {
			muB.Lock()
			receivedB = append(receivedB, evt.AggregateID())
			muB.Unlock()
			return nil
		})
		s.Require().NoError(err)
		defer unsubB()

		evtsPerPublisher := total / numPublishers
		var wg sync.WaitGroup
		for p := range numPublishers {
			wg.Add(1)
			go func(publisherID int) {
				defer wg.Done()
				for i := range evtsPerPublisher {
					seq := publisherID*evtsPerPublisher + i
					evt := newTestEvent(&s.Suite, s.fakeClock, fmt.Sprintf("seq-%05d", seq), "published")
					if pubErr := events.Publish(bus, s.ctx, evt); pubErr != nil {
						s.T().Errorf("publish falhou: %v", pubErr)
						return
					}
				}
			}(p)
		}

		wg.Wait()

		shutdownCtx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
		defer cancel()
		s.Require().NoError(bus.Close(shutdownCtx))

		s.Len(receivedA, total, "subscriberA deve receber %d eventos", total)
		s.Len(receivedB, total, "subscriberB deve receber %d eventos", total)
	})
}

// TestDropOnClose valida que eventos publicados após Close retornam ErrBusClosed.
func (s *BusIntegrationSuite) TestDropOnClose() {
	s.Run("deve retornar ErrBusClosed ao publicar após close e suportar close idempotente", func() {
		bus := events.NewBus(events.WithBufferSize(10))
		var processed atomic.Int64

		_, err := events.Subscribe(bus, func(_ context.Context, _ testEvent) error {
			processed.Add(1)
			return nil
		})
		s.Require().NoError(err)

		for i := range 5 {
			evt := newTestEvent(&s.Suite, s.fakeClock, fmt.Sprintf("agg-%d", i), "pre-close")
			s.Require().NoError(events.Publish(bus, s.ctx, evt))
		}

		shutdownCtx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
		defer cancel()
		s.Require().NoError(bus.Close(shutdownCtx))

		evt := newTestEvent(&s.Suite, s.fakeClock, "agg-post-close", "post-close")
		err = events.Publish(bus, s.ctx, evt)
		s.ErrorIs(err, events.ErrBusClosed)

		s.Require().NoError(bus.Close(shutdownCtx))
	})
}

// TestOrderPreservedPerSubscriber valida que a ordem FIFO é preservada por subscriber.
func (s *BusIntegrationSuite) TestOrderPreservedPerSubscriber() {
	s.Run("deve preservar ordem FIFO dos eventos por subscriber com publicador único", func() {
		const total = 100

		bus := events.NewBus(events.WithBufferSize(total + 10))
		var received []string
		var mu sync.Mutex

		unsub, err := events.Subscribe(bus, func(_ context.Context, evt testEvent) error {
			mu.Lock()
			received = append(received, evt.AggregateID())
			mu.Unlock()
			return nil
		})
		s.Require().NoError(err)
		defer unsub()

		for i := range total {
			evt := newTestEvent(&s.Suite, s.fakeClock, fmt.Sprintf("%05d", i), "ordered")
			s.Require().NoError(events.Publish(bus, s.ctx, evt))
		}

		shutdownCtx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
		defer cancel()
		s.Require().NoError(bus.Close(shutdownCtx))

		mu.Lock()
		defer mu.Unlock()

		s.Require().Len(received, total)
		for i, id := range received {
			expected := fmt.Sprintf("%05d", i)
			s.Equal(expected, id, "posição %d: esperado %s, recebido %s", i, expected, id)
		}
	})
}
