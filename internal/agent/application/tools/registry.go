package tools

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

var (
	ErrToolNameEmpty          = errors.New("agent.application.tools: tool name is empty")
	ErrToolDescriptionEmpty   = errors.New("agent.application.tools: tool description is empty")
	ErrToolSchemaVersionEmpty = errors.New("agent.application.tools: tool schema version is empty")
	ErrDuplicateToolName      = errors.New("agent.application.tools: duplicate tool name")
	ErrDuplicateIntentKind    = errors.New("agent.application.tools: duplicate intent kind")
	ErrEmptyRegistry          = errors.New("agent.application.tools: registry has no tools")
)

type ToolSpec struct {
	Name          string
	IntentKind    intent.Kind
	Description   string
	SchemaVersion string
	Timeout       time.Duration
	AuthzMode     AuthzMode
}

type Registry struct {
	specs    []ToolSpec
	byIntent map[intent.Kind]ToolSpec
}

func NewRegistry(specs ...ToolSpec) (*Registry, error) {
	if len(specs) == 0 {
		return nil, ErrEmptyRegistry
	}
	ordered := make([]ToolSpec, 0, len(specs))
	byIntent := make(map[intent.Kind]ToolSpec, len(specs))
	seenName := make(map[string]struct{}, len(specs))
	var errs []error
	for _, spec := range specs {
		name := strings.TrimSpace(spec.Name)
		if name == "" {
			errs = append(errs, fmt.Errorf("name=%q: %w", spec.Name, ErrToolNameEmpty))
			continue
		}
		if strings.TrimSpace(spec.Description) == "" {
			errs = append(errs, fmt.Errorf("name=%q: %w", name, ErrToolDescriptionEmpty))
			continue
		}
		if strings.TrimSpace(spec.SchemaVersion) == "" {
			errs = append(errs, fmt.Errorf("name=%q: %w", name, ErrToolSchemaVersionEmpty))
			continue
		}
		if _, exists := seenName[name]; exists {
			errs = append(errs, fmt.Errorf("name=%q: %w", name, ErrDuplicateToolName))
			continue
		}
		if _, exists := byIntent[spec.IntentKind]; exists {
			errs = append(errs, fmt.Errorf("name=%q intent=%q: %w", name, spec.IntentKind.String(), ErrDuplicateIntentKind))
			continue
		}
		seenName[name] = struct{}{}
		byIntent[spec.IntentKind] = spec
		ordered = append(ordered, spec)
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return &Registry{specs: ordered, byIntent: byIntent}, nil
}

func (r *Registry) SpecByIntent(kind intent.Kind) (ToolSpec, bool) {
	spec, ok := r.byIntent[kind]
	return spec, ok
}

func (r *Registry) Specs() []ToolSpec {
	out := make([]ToolSpec, len(r.specs))
	copy(out, r.specs)
	return out
}
