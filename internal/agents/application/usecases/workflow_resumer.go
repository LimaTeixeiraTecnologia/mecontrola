package usecases

import (
	"context"
	"fmt"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type WorkflowResumer interface {
	WorkflowID() string
	Resume(ctx context.Context, resourceID, threadID, message string) (bool, string, error)
}

type continueWorkflowFunc[S any] func(ctx context.Context, engine workflow.Engine[S], def workflow.Definition[S], key, message string) (bool, string, error)

type workflowResumer[S any] struct {
	workflowID string
	engine     workflow.Engine[S]
	def        workflow.Definition[S]
	keyFn      func(resourceID, threadID string) string
	continueFn continueWorkflowFunc[S]
}

func NewWorkflowResumer[S any](
	workflowID string,
	registry agent.WorkflowRegistry[S],
	engine workflow.Engine[S],
	keyFn func(resourceID, threadID string) string,
	continueFn continueWorkflowFunc[S],
) (WorkflowResumer, error) {
	def, ok := registry.Resolve(workflowID)
	if !ok {
		return nil, fmt.Errorf("usecases.workflow_resumer: workflow %s nao registrado", workflowID)
	}
	return &workflowResumer[S]{
		workflowID: workflowID,
		engine:     engine,
		def:        def,
		keyFn:      keyFn,
		continueFn: continueFn,
	}, nil
}

func (r *workflowResumer[S]) WorkflowID() string {
	return r.workflowID
}

func (r *workflowResumer[S]) Resume(ctx context.Context, resourceID, threadID, message string) (bool, string, error) {
	key := r.keyFn(resourceID, threadID)
	return r.continueFn(ctx, r.engine, r.def, key, message)
}
