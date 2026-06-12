package internal

import (
	"context"
	"sync"
	"time"
)

type Coalescer struct {
	mu      sync.Mutex
	timers  map[string]*timerEntry
	window  time.Duration
	timeout time.Duration
}

type timerEntry struct {
	timer *time.Timer
	fn    func(ctx context.Context)
}

func NewCoalescer(window, shutdownTimeout time.Duration) *Coalescer {
	return &Coalescer{
		timers:  make(map[string]*timerEntry),
		window:  window,
		timeout: shutdownTimeout,
	}
}

func (c *Coalescer) Schedule(key string, fn func(ctx context.Context)) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if e, ok := c.timers[key]; ok {
		e.fn = fn
		e.timer.Reset(c.window)
		return
	}

	t := time.AfterFunc(c.window, func() {
		c.mu.Lock()
		delete(c.timers, key)
		c.mu.Unlock()
		fn(context.Background())
	})
	c.timers[key] = &timerEntry{timer: t, fn: fn}
}

func (c *Coalescer) Pending() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.timers)
}

func (c *Coalescer) Stop(ctx context.Context) {
	c.mu.Lock()
	pending := make(map[string]*timerEntry, len(c.timers))
	for k, e := range c.timers {
		e.timer.Stop()
		pending[k] = e
	}
	c.timers = make(map[string]*timerEntry)
	c.mu.Unlock()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for _, e := range pending {
			e.fn(ctx)
		}
	}()

	deadline := time.NewTimer(c.timeout)
	defer deadline.Stop()
	select {
	case <-done:
	case <-deadline.C:
	case <-ctx.Done():
	}
}
