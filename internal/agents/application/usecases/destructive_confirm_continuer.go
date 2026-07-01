package usecases

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type DestructiveConfirmContinuer struct {
	engine workflow.Engine[workflows.ConfirmState]
	def    workflow.Definition[workflows.ConfirmState]
	o11y   observability.Observability
	total  observability.Counter
}

func NewDestructiveConfirmContinuer(
	engine workflow.Engine[workflows.ConfirmState],
	def workflow.Definition[workflows.ConfirmState],
	o11y observability.Observability,
) *DestructiveConfirmContinuer {
	total := o11y.Metrics().Counter(
		"agents_destructive_confirm_total",
		"Total de execucoes do gate de confirmacao destrutiva",
		"1",
	)
	return &DestructiveConfirmContinuer{
		engine: engine,
		def:    def,
		o11y:   o11y,
		total:  total,
	}
}

func (c *DestructiveConfirmContinuer) Continue(ctx context.Context, userID, message string) (bool, string, error) {
	ctx, span := c.o11y.Tracer().Start(ctx, "agents.usecase.destructive_confirm_continuer")
	defer span.End()

	key := workflows.DestructiveConfirmKey(userID)
	handled, reply, err := workflows.ContinueDestructiveConfirm(ctx, c.engine, c.def, key, message)
	if err != nil {
		span.RecordError(err)
		c.total.Add(ctx, 1, observability.String("result", "error"))
		return handled, reply, fmt.Errorf("agents.usecase.destructive_confirm_continuer: %w", err)
	}
	if handled {
		c.total.Add(ctx, 1, observability.String("result", "handled"))
	}
	return handled, reply, nil
}
