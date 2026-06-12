package internal_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	coalescerpkg "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/messaging/database/consumers/internal"
)

type CoalescerSuite struct {
	suite.Suite
}

func TestCoalescerSuite(t *testing.T) {
	suite.Run(t, new(CoalescerSuite))
}

func (s *CoalescerSuite) TestSameKey_Coalesces_To_OneCall() {
	var calls int64
	window := 100 * time.Millisecond
	c := coalescerpkg.NewCoalescer(window, 2*time.Second)

	for i := 0; i < 10; i++ {
		c.Schedule("user1:2026-06", func(_ context.Context) {
			atomic.AddInt64(&calls, 1)
		})
		time.Sleep(10 * time.Millisecond)
	}

	time.Sleep(window + 50*time.Millisecond)
	s.Equal(int64(1), atomic.LoadInt64(&calls))
}

func (s *CoalescerSuite) TestDistinctKeys_DoNotCoalesce() {
	var calls int64
	window := 100 * time.Millisecond
	c := coalescerpkg.NewCoalescer(window, 2*time.Second)

	for i := 0; i < 5; i++ {
		key := "user" + string(rune('0'+i)) + ":2026-06"
		c.Schedule(key, func(_ context.Context) {
			atomic.AddInt64(&calls, 1)
		})
	}

	time.Sleep(window + 50*time.Millisecond)
	s.Equal(int64(5), atomic.LoadInt64(&calls))
}

func (s *CoalescerSuite) TestStop_DrainsPendingTimers() {
	var calls int64
	window := 500 * time.Millisecond
	c := coalescerpkg.NewCoalescer(window, 2*time.Second)

	c.Schedule("user1:2026-06", func(_ context.Context) {
		atomic.AddInt64(&calls, 1)
	})
	c.Schedule("user2:2026-07", func(_ context.Context) {
		atomic.AddInt64(&calls, 1)
	})

	s.Equal(2, c.Pending())

	c.Stop(context.Background())

	s.Equal(0, c.Pending())
	s.Equal(int64(2), atomic.LoadInt64(&calls))
}

func (s *CoalescerSuite) TestStop_DuringPendingTimer_Drains() {
	var calls int64
	window := 2 * time.Second
	c := coalescerpkg.NewCoalescer(window, 5*time.Second)

	c.Schedule("user1:2026-06", func(_ context.Context) {
		atomic.AddInt64(&calls, 1)
	})

	s.Equal(1, c.Pending())
	c.Stop(context.Background())

	s.Equal(0, c.Pending())
	s.Equal(int64(1), atomic.LoadInt64(&calls))
}
