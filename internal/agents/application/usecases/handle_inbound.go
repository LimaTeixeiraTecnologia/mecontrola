package usecases

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

type HandleInbound struct {
	runtime agent.AgentRuntime
	o11y    observability.Observability
}

func NewHandleInbound(runtime agent.AgentRuntime, o11y observability.Observability) *HandleInbound {
	return &HandleInbound{runtime: runtime, o11y: o11y}
}

func (uc *HandleInbound) Execute(ctx context.Context, in input.InboundInput) (agent.Outcome, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "agents.usecase.handle_inbound")
	defer span.End()

	if err := in.Validate(); err != nil {
		return agent.Outcome{}, err
	}

	outcome, err := uc.runtime.Execute(ctx, agent.InboundRequest{
		ResourceID: in.ResourceID,
		ThreadID:   in.ThreadID,
		AgentID:    in.AgentID,
		Message:    in.Message,
		MessageID:  in.MessageID,
	})
	if err != nil {
		span.RecordError(err)
		return agent.Outcome{}, fmt.Errorf("agents.usecase.handle_inbound: %w", err)
	}

	return outcome, nil
}
