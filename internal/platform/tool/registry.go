package tool

import "fmt"

var ErrToolNotFound = fmt.Errorf("tool.registry: tool not found")

type Registry interface {
	Register(h ToolHandle)
	Resolve(id string) (ToolHandle, error)
}

type registry struct {
	handles map[string]ToolHandle
}

func NewRegistry() Registry {
	return &registry{
		handles: make(map[string]ToolHandle),
	}
}

func (r *registry) Register(h ToolHandle) {
	r.handles[h.ID()] = h
}

func (r *registry) Resolve(id string) (ToolHandle, error) {
	h, ok := r.handles[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrToolNotFound, id)
	}
	return h, nil
}
