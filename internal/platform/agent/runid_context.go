package agent

import (
	"context"

	"github.com/google/uuid"
)

type runIDContextKey struct{}

func WithRunID(ctx context.Context, runID uuid.UUID) context.Context {
	return context.WithValue(ctx, runIDContextKey{}, runID)
}

func RunIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	v, ok := ctx.Value(runIDContextKey{}).(uuid.UUID)
	return v, ok
}
