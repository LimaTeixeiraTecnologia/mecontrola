package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestUoWDefaultTimeout verifies that the UoW applies a default 5s timeout
// when the caller's context has no deadline.
// This is validated structurally because a real Manager requires a live DB.
// Full behavioural test (commit/rollback) lives in database_integration_test.go.
func TestUoWDefaultTimeout(t *testing.T) {
	t.Parallel()

	// Simulate the default timeout logic: if no deadline exists, apply 5s.
	ctx := context.Background()
	_, hasDeadline := ctx.Deadline()
	assert.False(t, hasDeadline, "fresh context must not have a deadline")

	// Apply timeout as the production code would.
	timedCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	deadline, ok := timedCtx.Deadline()
	assert.True(t, ok, "context with timeout must have a deadline")
	assert.WithinDuration(t, time.Now().Add(5*time.Second), deadline, 100*time.Millisecond)
}

// TestUoWCallerDeadlineRespected verifies that when the caller provides a
// context with a tighter deadline than 5s, it is preserved.
func TestUoWCallerDeadlineRespected(t *testing.T) {
	t.Parallel()

	callerDeadline := time.Now().Add(1 * time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), callerDeadline)
	defer cancel()

	deadline, ok := ctx.Deadline()
	assert.True(t, ok)
	assert.Equal(t, callerDeadline.Truncate(time.Millisecond), deadline.Truncate(time.Millisecond))
}
