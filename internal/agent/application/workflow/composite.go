package workflow

import (
	"context"
	"errors"
	"fmt"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

var (
	ErrWorkflowIDEmpty   = errors.New("agent.application.workflow: workflow id is empty")
	ErrNoTools           = errors.New("agent.application.workflow: workflow has no tools")
	ErrToolForKindNil    = errors.New("agent.application.workflow: tool for kind is nil")
	ErrKindNotHandled    = errors.New("agent.application.workflow: kind not handled by workflow")
	ErrDuplicateKindTool = errors.New("agent.application.workflow: kind bound to more than one tool")
)

type KindTool struct {
	Kind intent.Kind
	Tool tools.Tool
}

type composite struct {
	id     string
	guard  *WriteGuard
	byKind map[intent.Kind]tools.Tool
	order  []intent.Kind
}

func NewIntentWorkflow(id string, guard *WriteGuard, bindings ...KindTool) (IntentWorkflow, error) {
	if id == "" {
		return nil, ErrWorkflowIDEmpty
	}
	if len(bindings) == 0 {
		return nil, ErrNoTools
	}
	byKind := make(map[intent.Kind]tools.Tool, len(bindings))
	order := make([]intent.Kind, 0, len(bindings))
	var errs []error
	for _, binding := range bindings {
		if binding.Tool == nil {
			errs = append(errs, fmt.Errorf("kind=%q: %w", binding.Kind.String(), ErrToolForKindNil))
			continue
		}
		if _, exists := byKind[binding.Kind]; exists {
			errs = append(errs, fmt.Errorf("kind=%q: %w", binding.Kind.String(), ErrDuplicateKindTool))
			continue
		}
		byKind[binding.Kind] = binding.Tool
		order = append(order, binding.Kind)
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return &composite{id: id, guard: guard, byKind: byKind, order: order}, nil
}

func (c *composite) ID() string {
	return c.id
}

func (c *composite) Handles(kind intent.Kind) bool {
	_, ok := c.byKind[kind]
	return ok
}

func (c *composite) Execute(ctx context.Context, in tools.ToolInput) (tools.ToolResult, error) {
	kind := in.Intent.Kind()
	tool, ok := c.byKind[kind]
	if !ok {
		return tools.ToolResult{Kind: kind, Outcome: tools.OutcomeUsecaseError}, fmt.Errorf("workflow=%q kind=%q: %w", c.id, kind.String(), ErrKindNotHandled)
	}
	if !kind.IsWrite() || c.guard == nil {
		return tool.Execute(ctx, in)
	}
	decision, blocked, settle := c.guard.Apply(ctx, in)
	if decision == GuardShortCircuit {
		return blocked, nil
	}
	result, err := tool.Execute(ctx, in)
	if settle != nil && err == nil {
		settle(ctx, result.Outcome == tools.OutcomeRouted)
	}
	return result, err
}
