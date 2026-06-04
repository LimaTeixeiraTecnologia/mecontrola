package consumer

import (
	"context"
	"errors"
	"fmt"
)

var (
	errDuplicateEventType = errors.New("consumer: event type já registrado")
	errNilHandler         = errors.New("consumer: handler não pode ser nil")
	errUnknownEventType   = errors.New("consumer: event type desconhecido")
)

type Registry interface {
	Register(reg Registration) error
	Dispatch(ctx context.Context, eventType string, params map[string]string, body []byte) error
}

type registry struct {
	handlers map[string]Handler
}

func NewRegistry() Registry {
	return &registry{
		handlers: make(map[string]Handler),
	}
}

func (r *registry) Register(reg Registration) error {
	if reg.Handler == nil {
		return errNilHandler
	}
	if _, exists := r.handlers[reg.EventType]; exists {
		return fmt.Errorf("%w: %q", errDuplicateEventType, reg.EventType)
	}
	r.handlers[reg.EventType] = reg.Handler
	return nil
}

func (r *registry) Dispatch(ctx context.Context, eventType string, params map[string]string, body []byte) error {
	h, ok := r.handlers[eventType]
	if !ok {
		return fmt.Errorf("%w: %q", errUnknownEventType, eventType)
	}
	return h.Handle(ctx, params, body)
}
