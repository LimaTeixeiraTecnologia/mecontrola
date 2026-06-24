package workflow

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeStep(id string, fn func(intState) (intState, StepStatus, error)) Step[intState] {
	return NewStepFunc(id, func(_ context.Context, s intState) (StepOutput[intState], error) {
		next, status, err := fn(s)
		if err != nil {
			return StepOutput[intState]{State: s}, err
		}
		return StepOutput[intState]{State: next, Status: status}, nil
	})
}

func addStep(id string, delta int) Step[intState] {
	return makeStep(id, func(s intState) (intState, StepStatus, error) {
		return intState{value: s.value + delta}, StepStatusCompleted, nil
	})
}

func failStep(id string) Step[intState] {
	return makeStep(id, func(s intState) (intState, StepStatus, error) {
		return s, StepStatusFailed, errors.New("step failed")
	})
}

func suspendStep(id string) Step[intState] {
	return NewStepFunc(id, func(_ context.Context, s intState) (StepOutput[intState], error) {
		return StepOutput[intState]{
			State:   s,
			Status:  StepStatusSuspended,
			Suspend: &Suspension{Reason: SuspendAwaitingInput, Prompt: "confirm?"},
		}, nil
	})
}

func TestSequence_OrderAndStateThreading(t *testing.T) {
	seq := Sequence[intState]("seq",
		addStep("a", 1),
		addStep("b", 10),
		addStep("c", 100),
	)

	out, err := seq.Execute(context.Background(), intState{value: 0})
	require.NoError(t, err)
	assert.Equal(t, StepStatusCompleted, out.Status)
	assert.Equal(t, 111, out.State.value)
}

func TestSequence_ShortCircuitOnSuspend(t *testing.T) {
	seq := Sequence[intState]("seq",
		addStep("a", 1),
		suspendStep("b"),
		addStep("c", 100),
	)

	out, err := seq.Execute(context.Background(), intState{value: 0})
	require.NoError(t, err)
	assert.Equal(t, StepStatusSuspended, out.Status)
	assert.Equal(t, 1, out.State.value)
	assert.NotNil(t, out.Suspend)
	assert.Equal(t, SuspendAwaitingInput, out.Suspend.Reason)
}

func TestSequence_ShortCircuitOnFailed(t *testing.T) {
	seq := Sequence[intState]("seq",
		addStep("a", 1),
		failStep("b"),
		addStep("c", 100),
	)

	out, err := seq.Execute(context.Background(), intState{value: 0})
	require.Error(t, err)
	assert.Equal(t, 1, out.State.value)
}

func TestSequence_Empty(t *testing.T) {
	seq := Sequence[intState]("seq")
	out, err := seq.Execute(context.Background(), intState{value: 42})
	require.NoError(t, err)
	assert.Equal(t, StepStatusCompleted, out.Status)
	assert.Equal(t, 42, out.State.value)
}

func TestBranch_RoutesCorrectly(t *testing.T) {
	type branchState struct {
		key   string
		value int
	}

	branch := Branch[branchState]("branch",
		func(s branchState) string { return s.key },
		map[string]Step[branchState]{
			"add": NewStepFunc("add", func(_ context.Context, s branchState) (StepOutput[branchState], error) {
				return StepOutput[branchState]{State: branchState{key: s.key, value: s.value + 10}, Status: StepStatusCompleted}, nil
			}),
			"mul": NewStepFunc("mul", func(_ context.Context, s branchState) (StepOutput[branchState], error) {
				return StepOutput[branchState]{State: branchState{key: s.key, value: s.value * 10}, Status: StepStatusCompleted}, nil
			}),
		},
	)

	scenarios := []struct {
		name     string
		input    branchState
		expected int
		status   StepStatus
	}{
		{"add branch", branchState{key: "add", value: 5}, 15, StepStatusCompleted},
		{"mul branch", branchState{key: "mul", value: 5}, 50, StepStatusCompleted},
		{"missing route", branchState{key: "missing", value: 5}, 5, StepStatusSkipped},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			out, err := branch.Execute(context.Background(), s.input)
			require.NoError(t, err)
			assert.Equal(t, s.status, out.Status)
			assert.Equal(t, s.expected, out.State.value)
		})
	}
}

func TestParallel_DeterministicAggregation(t *testing.T) {
	parallel := Parallel[intState]("par",
		func(base intState, results []intState) intState {
			sum := 0
			for _, r := range results {
				sum += r.value
			}
			return intState{value: sum}
		},
		addStep("a", 1),
		addStep("b", 10),
		addStep("c", 100),
	)

	out, err := parallel.Execute(context.Background(), intState{value: 0})
	require.NoError(t, err)
	assert.Equal(t, StepStatusCompleted, out.Status)
	assert.Equal(t, 111, out.State.value)
}

func TestParallel_CancelledContext_NoLeak(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	blocker := NewStepFunc("blocker", func(ctx context.Context, s intState) (StepOutput[intState], error) {
		select {
		case <-ctx.Done():
			return StepOutput[intState]{State: s}, ctx.Err()
		case <-time.After(5 * time.Second):
			return StepOutput[intState]{State: s, Status: StepStatusCompleted}, nil
		}
	})

	parallel := Parallel[intState]("par",
		func(base intState, results []intState) intState { return base },
		blocker,
		blocker,
	)

	done := make(chan struct{})
	go func() {
		defer close(done)
		parallel.Execute(ctx, intState{value: 0}) //nolint:errcheck
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("parallel did not return after context cancellation — goroutine leak suspected")
	}
}

func TestParallel_ErrorPropagation(t *testing.T) {
	parallel := Parallel[intState]("par",
		func(base intState, results []intState) intState { return base },
		addStep("a", 1),
		failStep("b"),
	)

	_, err := parallel.Execute(context.Background(), intState{value: 0})
	assert.Error(t, err)
}

func TestRetry_SucceedsOnFirstAttempt(t *testing.T) {
	step := addStep("ok", 5)
	retryStep := Retry[intState](step, RetryPolicy{MaxAttempts: 3, BaseBackoff: time.Millisecond, MaxBackoff: 10 * time.Millisecond})

	out, err := retryStep.Execute(context.Background(), intState{value: 0})
	require.NoError(t, err)
	assert.Equal(t, 5, out.State.value)
}

func TestRetry_ExhaustsAttempts(t *testing.T) {
	attempts := 0
	failing := NewStepFunc("failing", func(_ context.Context, s intState) (StepOutput[intState], error) {
		attempts++
		return StepOutput[intState]{State: s}, fmt.Errorf("attempt %d failed", attempts)
	})

	maxAttempts := 3
	retried := Retry[intState](failing, RetryPolicy{
		MaxAttempts: maxAttempts,
		BaseBackoff: time.Millisecond,
		MaxBackoff:  5 * time.Millisecond,
	})

	_, err := retried.Execute(context.Background(), intState{value: 0})
	assert.Error(t, err)
	assert.Equal(t, maxAttempts, attempts)
	assert.Contains(t, err.Error(), "retry exhausted")
}

func TestRetry_SucceedsAfterRetries(t *testing.T) {
	attempts := 0
	step := NewStepFunc("flaky", func(_ context.Context, s intState) (StepOutput[intState], error) {
		attempts++
		if attempts < 3 {
			return StepOutput[intState]{State: s}, errors.New("temporary error")
		}
		return StepOutput[intState]{State: intState{value: s.value + 1}, Status: StepStatusCompleted}, nil
	})

	retried := Retry[intState](step, RetryPolicy{
		MaxAttempts: 5,
		BaseBackoff: time.Millisecond,
		MaxBackoff:  10 * time.Millisecond,
	})

	out, err := retried.Execute(context.Background(), intState{value: 0})
	require.NoError(t, err)
	assert.Equal(t, 1, out.State.value)
	assert.Equal(t, 3, attempts)
}

func TestRetry_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	callCount := 0
	step := NewStepFunc("step", func(_ context.Context, s intState) (StepOutput[intState], error) {
		callCount++
		cancel()
		return StepOutput[intState]{State: s}, errors.New("error")
	})

	retried := Retry[intState](step, RetryPolicy{
		MaxAttempts: 10,
		BaseBackoff: 10 * time.Millisecond,
		MaxBackoff:  100 * time.Millisecond,
	})

	_, err := retried.Execute(ctx, intState{value: 0})
	assert.Error(t, err)
	assert.Equal(t, 1, callCount)
}
