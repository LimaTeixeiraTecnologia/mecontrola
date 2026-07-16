package scorer

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"
)

const defaultWorkers = 8

type ScorerEntry struct {
	scorer   Scorer
	sampling Sampling
}

func (e ScorerEntry) ScorerID() string {
	return e.scorer.ID()
}

type observeJob struct {
	runID  uuid.UUID
	sample RunSample
}

type runnerMetrics struct {
	runsTotal   observability.Counter
	runDuration observability.Histogram
}

type scorerRunner struct {
	scorers []ScorerEntry
	store   ResultStore
	jobs    chan observeJob
	workers int
	wg      sync.WaitGroup
	o11y    observability.Observability
	metrics runnerMetrics
}

type RunnerOption func(*scorerRunner)

func WithWorkers(n int) RunnerOption {
	return func(r *scorerRunner) {
		count := normalizeWorkers(n)
		r.workers = count
		r.jobs = make(chan observeJob, count*4)
	}
}

func normalizeWorkers(n int) int {
	if n <= 0 {
		return defaultWorkers
	}
	return n
}

func NewScorerRunner(
	scorers []ScorerEntry,
	store ResultStore,
	o11y observability.Observability,
	opts ...RunnerOption,
) ScorerRunner {
	r := &scorerRunner{
		scorers: scorers,
		store:   store,
		jobs:    make(chan observeJob, defaultWorkers*4),
		workers: defaultWorkers,
		o11y:    o11y,
		metrics: runnerMetrics{
			runsTotal:   o11y.Metrics().Counter("scorer_runs_total", "Total scorer runs", "1"),
			runDuration: o11y.Metrics().Histogram("scorer_duration_seconds", "Scorer run duration", "s"),
		},
	}
	for _, opt := range opts {
		opt(r)
	}

	for i := 0; i < r.workers; i++ {
		r.wg.Add(1)
		go r.work()
	}

	return r
}

func NewScorerEntry(s Scorer, sampling Sampling) ScorerEntry {
	return ScorerEntry{scorer: s, sampling: sampling}
}

func (r *scorerRunner) Observe(ctx context.Context, runID uuid.UUID, s RunSample) {
	select {
	case r.jobs <- observeJob{runID: runID, sample: s}:
	default:
		r.metrics.runsTotal.Add(ctx, 1,
			observability.String("outcome", "dropped"),
		)
	}
}

func (r *scorerRunner) Shutdown(ctx context.Context) {
	close(r.jobs)
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
	}
}

func (r *scorerRunner) work() {
	defer r.wg.Done()
	for job := range r.jobs {
		r.processJob(job)
	}
}

func (r *scorerRunner) processJob(job observeJob) {
	ctx := context.Background()
	for _, entry := range r.scorers {
		sampled := r.shouldSample(entry.sampling)

		start := time.Now()
		outcome := "success"

		if !sampled {
			r.metrics.runsTotal.Add(ctx, 1,
				observability.String("scorer_id", entry.scorer.ID()),
				observability.String("kind", entry.scorer.Kind().String()),
				observability.String("outcome", "skipped"),
			)
			continue
		}

		result, err := entry.scorer.Score(ctx, job.sample)
		elapsed := time.Since(start).Seconds()

		if err != nil {
			outcome = "error"
			r.metrics.runsTotal.Add(ctx, 1,
				observability.String("scorer_id", entry.scorer.ID()),
				observability.String("kind", entry.scorer.Kind().String()),
				observability.String("outcome", outcome),
			)
			r.metrics.runDuration.Record(ctx, elapsed,
				observability.String("scorer_id", entry.scorer.ID()),
				observability.String("kind", entry.scorer.Kind().String()),
			)
			continue
		}

		r.metrics.runsTotal.Add(ctx, 1,
			observability.String("scorer_id", entry.scorer.ID()),
			observability.String("kind", entry.scorer.Kind().String()),
			observability.String("outcome", outcome),
		)
		r.metrics.runDuration.Record(ctx, elapsed,
			observability.String("scorer_id", entry.scorer.ID()),
			observability.String("kind", entry.scorer.Kind().String()),
		)

		if insertErr := r.store.Insert(ctx, ScorerResult{
			ID:        uuid.New(),
			RunID:     job.runID,
			ScorerID:  entry.scorer.ID(),
			Kind:      entry.scorer.Kind(),
			Score:     result.Score,
			Reason:    result.Reason,
			Metadata:  result.Metadata,
			Sampled:   true,
			CreatedAt: time.Now().UTC(),
		}); insertErr != nil {
			r.metrics.runsTotal.Add(ctx, 1,
				observability.String("scorer_id", entry.scorer.ID()),
				observability.String("kind", entry.scorer.Kind().String()),
				observability.String("outcome", "persist_error"),
			)
			r.o11y.Logger().Error(ctx, "scorer.runner.persist.failed",
				observability.String("scorer_id", entry.scorer.ID()),
				observability.Error(insertErr),
			)
		}
	}
}

func (r *scorerRunner) shouldSample(s Sampling) bool {
	switch s.Type {
	case SamplingTypeAlways:
		return true
	case SamplingTypeNever:
		return false
	case SamplingTypeRatio:
		return rand.Float64() < s.Ratio
	default:
		return false
	}
}
