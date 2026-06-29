package agent

import "context"

type NoopHooks struct{}

func (NoopHooks) BeforeExecute(ctx context.Context, _ string, _ Request) context.Context {
	return ctx
}

func (NoopHooks) AfterExecute(_ context.Context, _ string, _ Result, _ error) {}

func (NoopHooks) BeforeTool(ctx context.Context, _, _ string) context.Context {
	return ctx
}

func (NoopHooks) AfterTool(_ context.Context, _, _ string, _ []byte, _ error) {}
