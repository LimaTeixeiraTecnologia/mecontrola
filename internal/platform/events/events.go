package events

import (
	"context"
	"errors"
)

var (
	ErrHandlerAlreadyRegistered = errors.New("events: handler já registrado para este tipo")
	ErrEventNil                 = errors.New("events: event não pode ser nil")
	ErrHandlerNil               = errors.New("events: handler não pode ser nil")
	ErrEventTypeEmpty           = errors.New("events: event type não pode ser vazio")
)

type Event interface {
	GetEventType() string
	GetPayload() any
}

type Handler interface {
	Handle(ctx context.Context, event Event) error
}

type Dispatcher interface {
	Register(eventType string, handler Handler) error
	Dispatch(ctx context.Context, event Event) error
	Remove(eventType string, handler Handler) error
	Has(eventType string, handler Handler) bool
	HandlersOf(eventType string) []Handler
	Clear()
}
