package workflow

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

type Workflow interface {
	ID() string
	Handles(kind intent.Kind) bool
	Execute(ctx context.Context, in tools.ToolInput) (tools.ToolResult, error)
}
