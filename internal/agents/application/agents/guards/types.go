package guards

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

type GuardDecision struct {
	Handled bool
	Result  agent.Result
}

type PreGuard interface {
	Name() string
	Inspect(ctx context.Context, in agent.Request) GuardDecision
}

type PostGuard interface {
	Name() string
	Inspect(ctx context.Context, in agent.Request, out agent.Result) GuardDecision
}
