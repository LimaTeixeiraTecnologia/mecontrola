package consumer

import "context"

type Handler interface {
	Handle(ctx context.Context, params map[string]string, body []byte) error
}

type HandlerFunc func(ctx context.Context, params map[string]string, body []byte) error

func (f HandlerFunc) Handle(ctx context.Context, params map[string]string, body []byte) error {
	return f(ctx, params, body)
}

type Message struct {
	EventType string
	Params    map[string]string
	Body      []byte
}

type Source interface {
	Start(ctx context.Context, deliver func(context.Context, Message) error) error
	Stop(ctx context.Context) error
}

type Runner interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}
