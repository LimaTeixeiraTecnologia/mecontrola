package workflow

import (
	"errors"
	"fmt"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

var (
	ErrEmptyRegistry      = errors.New("agent.application.workflow: registry has no workflows")
	ErrNilWorkflow        = errors.New("agent.application.workflow: workflow is nil")
	ErrDuplicateWorkflow  = errors.New("agent.application.workflow: duplicate workflow id")
	ErrDuplicateKindOwner = errors.New("agent.application.workflow: intent kind handled by more than one workflow")
)

type IntentRegistry struct {
	workflows []IntentWorkflow
	byKind    map[intent.Kind]IntentWorkflow
	byID      map[string]IntentWorkflow
}

func NewIntentRegistry(kinds []intent.Kind, workflows ...IntentWorkflow) (*IntentRegistry, error) {
	if len(workflows) == 0 {
		return nil, ErrEmptyRegistry
	}
	byID := make(map[string]IntentWorkflow, len(workflows))
	byKind := make(map[intent.Kind]IntentWorkflow, len(kinds))
	ordered := make([]IntentWorkflow, 0, len(workflows))
	var errs []error
	for _, wf := range workflows {
		if wf == nil {
			errs = append(errs, ErrNilWorkflow)
			continue
		}
		if _, exists := byID[wf.ID()]; exists {
			errs = append(errs, fmt.Errorf("id=%q: %w", wf.ID(), ErrDuplicateWorkflow))
			continue
		}
		byID[wf.ID()] = wf
		ordered = append(ordered, wf)
	}
	for _, kind := range kinds {
		owner, ok := resolveOwner(ordered, kind)
		if !ok {
			continue
		}
		if existing, exists := byKind[kind]; exists && existing.ID() != owner.ID() {
			errs = append(errs, fmt.Errorf("kind=%q owners=%q,%q: %w", kind.String(), existing.ID(), owner.ID(), ErrDuplicateKindOwner))
			continue
		}
		byKind[kind] = owner
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return &IntentRegistry{workflows: ordered, byKind: byKind, byID: byID}, nil
}

func resolveOwner(workflows []IntentWorkflow, kind intent.Kind) (IntentWorkflow, bool) {
	for _, wf := range workflows {
		if wf.Handles(kind) {
			return wf, true
		}
	}
	return nil, false
}

func (r *IntentRegistry) Resolve(kind intent.Kind) (IntentWorkflow, bool) {
	wf, ok := r.byKind[kind]
	return wf, ok
}

func (r *IntentRegistry) Workflows() []IntentWorkflow {
	out := make([]IntentWorkflow, len(r.workflows))
	copy(out, r.workflows)
	return out
}
