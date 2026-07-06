package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

type StaleSuspendedReaper struct {
	store    Store
	workflow string
	maxAge   time.Duration
	batch    int
	reaped   observability.Counter
}

func NewStaleSuspendedReaper(store Store, workflowName string, maxAge time.Duration, batch int, o11y observability.Observability) *StaleSuspendedReaper {
	return &StaleSuspendedReaper{
		store:    store,
		workflow: workflowName,
		maxAge:   maxAge,
		batch:    batch,
		reaped: o11y.Metrics().Counter(
			"workflow_stale_suspended_reaped_total",
			"Total de runs suspensos expirados marcados como failed pelo reaper",
			"1",
		),
	}
}

func (r *StaleSuspendedReaper) Reap(ctx context.Context) (int64, error) {
	cutoff := time.Now().UTC().Add(-r.maxAge)
	snaps, err := r.store.ListSuspended(ctx, r.workflow, cutoff, r.batch)
	if err != nil {
		return 0, fmt.Errorf("workflow: reaper: listar suspensos: %w", err)
	}

	var count int64
	for _, snap := range snaps {
		ended := time.Now().UTC()
		snap.Status = RunStatusFailed
		snap.EndedAt = &ended
		snap.LastError = "workflow: suspenso alem do TTL, expirado pelo reaper"
		if saveErr := r.store.Save(ctx, snap, snap.Version); saveErr != nil {
			continue
		}
		count++
	}

	if count > 0 {
		r.reaped.Add(ctx, count,
			observability.String("workflow", r.workflow),
			observability.String("status", "failed"),
		)
	}
	return count, nil
}
