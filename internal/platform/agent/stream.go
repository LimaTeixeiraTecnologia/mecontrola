package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
)

type resultStream struct {
	ts      llm.TokenStream
	decoder StructuredDecoder
	hooks   Hooks
	agentID string
	out     chan string
	done    chan struct{}
	mu      sync.Mutex
	result  Result
	err     error
	buf     strings.Builder
}

func newResultStream(ctx context.Context, ts llm.TokenStream, decoder StructuredDecoder, hooks Hooks, agentID string) *resultStream {
	rs := &resultStream{
		ts:      ts,
		decoder: decoder,
		hooks:   hooks,
		agentID: agentID,
		out:     make(chan string, 64),
		done:    make(chan struct{}),
	}
	go rs.drain(ctx)
	return rs
}

func (rs *resultStream) drain(ctx context.Context) {
	defer close(rs.done)
	defer close(rs.out)
	for delta := range rs.ts.Deltas() {
		rs.buf.WriteString(delta)
		select {
		case rs.out <- delta:
		case <-ctx.Done():
			rs.finalize(ctx, Result{}, ctx.Err())
			return
		}
	}
	if err := rs.ts.Err(); err != nil {
		rs.finalize(ctx, Result{}, fmt.Errorf("agent.stream: provider: %w", err))
		return
	}
	raw := []byte(rs.buf.String())
	if rs.decoder != nil {
		if err := rs.decoder.Validate(raw); err != nil {
			rs.finalize(ctx, Result{}, fmt.Errorf("%w: %w", ErrContractNotMet, err))
			return
		}
	}
	rs.finalize(ctx, Result{Content: rs.buf.String(), RawJSON: raw, Mode: ExecutionModeStream}, nil)
}

func (rs *resultStream) finalize(ctx context.Context, result Result, err error) {
	rs.mu.Lock()
	rs.result = result
	rs.err = err
	rs.mu.Unlock()
	if rs.hooks != nil {
		rs.hooks.AfterExecute(ctx, rs.agentID, result, err)
	}
}

func (rs *resultStream) Deltas() <-chan string {
	return rs.out
}

func (rs *resultStream) Result(ctx context.Context) (Result, error) {
	for {
		select {
		case <-rs.done:
			rs.mu.Lock()
			res, err := rs.result, rs.err
			rs.mu.Unlock()
			return res, err
		case <-ctx.Done():
			return Result{}, ctx.Err()
		case _, ok := <-rs.out:
			if !ok {
				<-rs.done
				rs.mu.Lock()
				res, err := rs.result, rs.err
				rs.mu.Unlock()
				return res, err
			}
		}
	}
}
