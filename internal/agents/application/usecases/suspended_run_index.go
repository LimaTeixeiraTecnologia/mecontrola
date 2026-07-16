package usecases

import (
	"context"
	"errors"
	"fmt"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

var ErrMultipleSuspendedRuns = errors.New("usecases.suspended_run_index: mais de um run suspenso para a mesma thread")

type SuspendedRunIndex struct {
	store       workflow.Store
	workflowIDs []string
}

func NewSuspendedRunIndex(store workflow.Store, workflowIDs ...string) *SuspendedRunIndex {
	ids := make([]string, len(workflowIDs))
	copy(ids, workflowIDs)
	return &SuspendedRunIndex{store: store, workflowIDs: ids}
}

func (idx *SuspendedRunIndex) Resolve(ctx context.Context, resourceID, threadID string) (string, bool, error) {
	found := ""
	for _, workflowID := range idx.workflowIDs {
		key := workflows.CorrelationKey(resourceID, threadID, workflowID)
		snap, ok, err := idx.store.Load(ctx, workflowID, key)
		if err != nil {
			return "", false, fmt.Errorf("usecases.suspended_run_index: load %s: %w", workflowID, err)
		}
		if !ok || snap.Status != workflow.RunStatusSuspended {
			continue
		}
		if found != "" {
			return "", false, fmt.Errorf("%w: resource=%s thread=%s workflows=%s,%s", ErrMultipleSuspendedRuns, resourceID, threadID, found, workflowID)
		}
		found = workflowID
	}
	if found == "" {
		return "", false, nil
	}
	return found, true, nil
}
