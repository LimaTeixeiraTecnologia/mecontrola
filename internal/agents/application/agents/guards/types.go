package guards

import (
	"context"
	"log/slog"

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

func logShortcutInvokeError(ctx context.Context, guard string, err error) {
	slog.WarnContext(ctx, "agents.guards: invoke do shortcut falhou; delegando ao llm",
		"guard", guard,
		"error", err.Error(),
	)
}
