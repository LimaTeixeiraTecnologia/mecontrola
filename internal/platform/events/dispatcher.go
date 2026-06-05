package events

import (
	"context"
	"slices"
	"sync"
)

type Option func(*dispatcher)

func WithCapacity(capacity int) Option {
	return func(d *dispatcher) {
		d.capacity = capacity
	}
}

type dispatcher struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
	capacity int
}

func NewDispatcher(opts ...Option) Dispatcher {
	d := &dispatcher{}
	for _, opt := range opts {
		opt(d)
	}
	d.handlers = make(map[string][]Handler, d.capacity)
	return d
}

func (d *dispatcher) Register(eventType string, handler Handler) error {
	if eventType == "" {
		return ErrEventTypeEmpty
	}
	if handler == nil {
		return ErrHandlerNil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if slices.Contains(d.handlers[eventType], handler) {
		return ErrHandlerAlreadyRegistered
	}
	d.handlers[eventType] = append(d.handlers[eventType], handler)
	return nil
}

func (d *dispatcher) Dispatch(ctx context.Context, event Event) error {
	if event == nil {
		return ErrEventNil
	}
	if event.GetEventType() == "" {
		return ErrEventTypeEmpty
	}
	d.mu.RLock()
	src := d.handlers[event.GetEventType()]
	snapshot := make([]Handler, len(src))
	copy(snapshot, src)
	d.mu.RUnlock()

	for _, h := range snapshot {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err := h.Handle(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

func (d *dispatcher) Remove(eventType string, handler Handler) error {
	if eventType == "" || handler == nil {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	hs, ok := d.handlers[eventType]
	if !ok {
		return nil
	}
	idx := slices.Index(hs, handler)
	if idx < 0 {
		return nil
	}
	d.handlers[eventType] = slices.Delete(hs, idx, idx+1)
	if len(d.handlers[eventType]) == 0 {
		delete(d.handlers, eventType)
	}
	return nil
}

func (d *dispatcher) Has(eventType string, handler Handler) bool {
	if eventType == "" || handler == nil {
		return false
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	return slices.Contains(d.handlers[eventType], handler)
}

func (d *dispatcher) HandlersOf(eventType string) []Handler {
	d.mu.RLock()
	defer d.mu.RUnlock()
	src := d.handlers[eventType]
	snapshot := make([]Handler, len(src))
	copy(snapshot, src)
	return snapshot
}

func (d *dispatcher) Clear() {
	d.mu.Lock()
	defer d.mu.Unlock()
	clear(d.handlers)
}
