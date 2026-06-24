package workflow

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

type sequenceStep[S any] struct {
	id    string
	steps []Step[S]
}

func Sequence[S any](id string, steps ...Step[S]) Step[S] {
	return &sequenceStep[S]{id: id, steps: steps}
}

func (s *sequenceStep[S]) ID() string { return s.id }

func (s *sequenceStep[S]) Execute(ctx context.Context, state S) (StepOutput[S], error) {
	current := state
	for _, step := range s.steps {
		out, err := step.Execute(ctx, current)
		if err != nil {
			return StepOutput[S]{State: current}, fmt.Errorf("%s: %w", step.ID(), err)
		}
		current = out.State
		if out.Status == StepStatusSuspended {
			return StepOutput[S]{State: current, Status: StepStatusSuspended, Suspend: out.Suspend}, nil
		}
		if out.Status == StepStatusFailed {
			return StepOutput[S]{State: current, Status: StepStatusFailed}, nil
		}
	}
	return StepOutput[S]{State: current, Status: StepStatusCompleted}, nil
}

type branchStep[S any] struct {
	id     string
	decide func(S) string
	routes map[string]Step[S]
}

func Branch[S any](id string, decide func(S) string, routes map[string]Step[S]) Step[S] {
	return &branchStep[S]{id: id, decide: decide, routes: routes}
}

func (b *branchStep[S]) ID() string { return b.id }

func (b *branchStep[S]) Execute(ctx context.Context, state S) (StepOutput[S], error) {
	key := b.decide(state)
	step, ok := b.routes[key]
	if !ok {
		return StepOutput[S]{State: state, Status: StepStatusSkipped}, nil
	}
	out, err := step.Execute(ctx, state)
	if err != nil {
		return StepOutput[S]{State: state}, fmt.Errorf("%s: %w", step.ID(), err)
	}
	return out, nil
}

type parallelStep[S any] struct {
	id    string
	merge func(base S, results []S) S
	steps []Step[S]
}

func Parallel[S any](id string, merge func(base S, results []S) S, steps ...Step[S]) Step[S] {
	return &parallelStep[S]{id: id, merge: merge, steps: steps}
}

func (p *parallelStep[S]) ID() string { return p.id }

func (p *parallelStep[S]) Execute(ctx context.Context, state S) (StepOutput[S], error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	type result struct {
		state S
		err   error
	}

	results := make([]result, len(p.steps))
	var wg sync.WaitGroup

	for i, step := range p.steps {
		wg.Add(1)
		go func(idx int, s Step[S]) {
			defer wg.Done()
			out, err := s.Execute(ctx, state)
			results[idx] = result{state: out.State, err: err}
		}(i, step)
	}

	wg.Wait()

	var errs []error
	var states []S
	for _, r := range results {
		if r.err != nil {
			errs = append(errs, r.err)
			continue
		}
		states = append(states, r.state)
	}

	if len(errs) > 0 {
		return StepOutput[S]{State: state}, errors.Join(errs...)
	}

	merged := p.merge(state, states)
	return StepOutput[S]{State: merged, Status: StepStatusCompleted}, nil
}

type RetryPolicy struct {
	MaxAttempts int
	BaseBackoff time.Duration
	MaxBackoff  time.Duration
}

type retryStep[S any] struct {
	step   Step[S]
	policy RetryPolicy
	rng    *rand.Rand
	mu     sync.Mutex
}

func Retry[S any](step Step[S], policy RetryPolicy) Step[S] {
	return &retryStep[S]{
		step:   step,
		policy: policy,
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())), //nolint:gosec
	}
}

func (r *retryStep[S]) ID() string { return r.step.ID() }

func (r *retryStep[S]) Execute(ctx context.Context, state S) (StepOutput[S], error) {
	var lastErr error
	for attempt := 1; attempt <= r.policy.MaxAttempts; attempt++ {
		out, err := r.step.Execute(ctx, state)
		if err == nil {
			return out, nil
		}
		lastErr = err
		if attempt == r.policy.MaxAttempts {
			break
		}
		backoff := r.calcBackoff(attempt)
		select {
		case <-ctx.Done():
			return StepOutput[S]{State: state}, ctx.Err()
		case <-time.After(backoff):
		}
	}
	return StepOutput[S]{State: state}, fmt.Errorf("retry exhausted after %d attempts: %w", r.policy.MaxAttempts, lastErr)
}

func (r *retryStep[S]) calcBackoff(attempt int) time.Duration {
	exp := r.policy.BaseBackoff
	for i := 1; i < attempt; i++ {
		exp *= 2
		if exp > r.policy.MaxBackoff {
			exp = r.policy.MaxBackoff
			break
		}
	}
	r.mu.Lock()
	jitter := time.Duration(r.rng.Int63n(int64(exp/2 + 1)))
	r.mu.Unlock()
	result := exp/2 + jitter
	if result > r.policy.MaxBackoff {
		return r.policy.MaxBackoff
	}
	return result
}
