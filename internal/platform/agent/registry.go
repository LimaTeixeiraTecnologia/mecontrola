package agent

import (
	"fmt"
	"sync"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type agentRegistry struct {
	mu     sync.RWMutex
	agents map[string]Agent
}

func NewAgentRegistry() AgentRegistry {
	return &agentRegistry{agents: make(map[string]Agent)}
}

func (r *agentRegistry) Register(a Agent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[a.ID()] = a
}

func (r *agentRegistry) Resolve(id string) (Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.agents[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrAgentNotFound, id)
	}
	return a, nil
}

type workflowRegistry[S any] struct {
	mu   sync.RWMutex
	defs map[string]workflow.Definition[S]
}

func NewWorkflowRegistry[S any]() MutableWorkflowRegistry[S] {
	return &workflowRegistry[S]{defs: make(map[string]workflow.Definition[S])}
}

func (r *workflowRegistry[S]) Register(def workflow.Definition[S]) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.defs[def.ID] = def
}

func (r *workflowRegistry[S]) Resolve(agentID string) (workflow.Definition[S], bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, ok := r.defs[agentID]
	return def, ok
}
