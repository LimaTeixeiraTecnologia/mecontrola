package llm

import "context"

type Provider interface {
	Slug() string
	Complete(ctx context.Context, req Request) (Response, error)
	Stream(ctx context.Context, req Request) (TokenStream, error)
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

type TokenStream interface {
	Deltas() <-chan string
	Close() error
	Err() error
}
