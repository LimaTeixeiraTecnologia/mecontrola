package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

type AlertContextExpirer interface {
	PurgeExpired(ctx context.Context, resourceID string, now time.Time) error
}

type HandleInbound struct {
	runtime      agent.AgentRuntime
	alertContext AlertContextExpirer
	o11y         observability.Observability
}

func NewHandleInbound(runtime agent.AgentRuntime, alertContext AlertContextExpirer, o11y observability.Observability) *HandleInbound {
	return &HandleInbound{runtime: runtime, alertContext: alertContext, o11y: o11y}
}

func (uc *HandleInbound) Execute(ctx context.Context, in input.InboundInput) (agent.Outcome, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "agents.usecase.handle_inbound")
	defer span.End()

	if err := in.Validate(); err != nil {
		return agent.Outcome{}, err
	}

	uc.purgeExpiredAlertContext(ctx, in.ResourceID)

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

func (uc *HandleInbound) purgeExpiredAlertContext(ctx context.Context, resourceID string) {
	if uc.alertContext == nil {
		return
	}
	if err := uc.alertContext.PurgeExpired(ctx, resourceID, time.Now().UTC()); err != nil {
		uc.o11y.Logger().Warn(ctx, "agents.usecase.handle_inbound.alert_context_purge_failed",
			observability.Error(err),
		)
	}
}
