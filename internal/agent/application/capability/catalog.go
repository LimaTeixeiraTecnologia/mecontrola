package capability

import (
	"errors"
	"fmt"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

var (
	ErrCapabilityWorkflowIDEmpty = errors.New("agent.application.capability: workflow id is empty")
	ErrCapabilityDuplicateID     = errors.New("agent.application.capability: duplicate id")
	ErrCapabilityDuplicateKind   = errors.New("agent.application.capability: duplicate kind")
)

const workflowConversational = "conversational"

type Catalog struct {
	byKind map[intent.Kind]CapabilitySpec
	specs  []CapabilitySpec
}

func NewCatalog(specs ...CapabilitySpec) (*Catalog, error) {
	byKind := make(map[intent.Kind]CapabilitySpec, len(specs))
	byID := make(map[string]struct{}, len(specs))
	ordered := make([]CapabilitySpec, 0, len(specs))
	var errs []error
	for _, spec := range specs {
		if _, exists := byID[spec.ID]; exists {
			errs = append(errs, fmt.Errorf("id=%q: %w", spec.ID, ErrCapabilityDuplicateID))
		} else {
			byID[spec.ID] = struct{}{}
		}
		if existing, exists := byKind[spec.Kind]; exists {
			errs = append(errs, fmt.Errorf("kind=%q ids=%q,%q: %w", spec.Kind.String(), existing.ID, spec.ID, ErrCapabilityDuplicateKind))
		}
		if !spec.Mode.IsValid() {
			errs = append(errs, fmt.Errorf("id=%q kind=%q field=mode: %w", spec.ID, spec.Kind.String(), ErrCapabilityModeInvalid))
		}
		if spec.WorkflowID == "" {
			errs = append(errs, fmt.Errorf("id=%q kind=%q field=workflow_id: %w", spec.ID, spec.Kind.String(), ErrCapabilityWorkflowIDEmpty))
		}
		cloned := cloneSpec(spec)
		byKind[spec.Kind] = cloned
		ordered = append(ordered, cloned)
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return &Catalog{byKind: byKind, specs: ordered}, nil
}

func (c *Catalog) Lookup(kind intent.Kind) (CapabilitySpec, bool) {
	if c == nil {
		return CapabilitySpec{}, false
	}
	spec, ok := c.byKind[kind]
	if !ok {
		return CapabilitySpec{}, false
	}
	return cloneSpec(spec), true
}

func (c *Catalog) List() []CapabilitySpec {
	if c == nil {
		return nil
	}
	out := make([]CapabilitySpec, 0, len(c.specs))
	for _, spec := range c.specs {
		out = append(out, cloneSpec(spec))
	}
	return out
}

func (c *Catalog) Classify(kind intent.Kind) (workflow, tool string) {
	spec, ok := c.Lookup(kind)
	if !ok {
		return workflowConversational, ""
	}
	return spec.WorkflowID, spec.ToolName
}

func cloneSpec(spec CapabilitySpec) CapabilitySpec {
	if spec.Channels == nil {
		return spec
	}
	spec.Channels = append([]string(nil), spec.Channels...)
	return spec
}
