package workflow

import "context"

type Runtime = any

type runtimeKey struct{}

func WithRuntime(ctx context.Context, rc Runtime) context.Context {
	return context.WithValue(ctx, runtimeKey{}, rc)
}

func RuntimeFrom(ctx context.Context) (Runtime, bool) {
	rc := ctx.Value(runtimeKey{})
	return rc, rc != nil
}
