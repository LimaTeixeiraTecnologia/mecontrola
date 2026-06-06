package consumers

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type projectSubscriptionEventUseCase interface {
	Execute(ctx context.Context, in input.ProjectSubscriptionEvent) error
}

type SubscriptionEventProjector struct {
	usecase projectSubscriptionEventUseCase
	o11y    observability.Observability
}

func NewSubscriptionEventProjector(
	uc projectSubscriptionEventUseCase,
	o11y observability.Observability,
) *SubscriptionEventProjector {
	return &SubscriptionEventProjector{usecase: uc, o11y: o11y}
}

func (p *SubscriptionEventProjector) Handle(ctx context.Context, event events.Event) error {
	ctx, span := p.o11y.Tracer().Start(ctx, "identity.projector.subscription_event.handle")
	defer span.End()

	env, ok := event.GetPayload().(outbox.Envelope)
	if !ok {
		return fmt.Errorf("identity.projector: unexpected payload type %T", event.GetPayload())
	}

	return p.usecase.Execute(ctx, input.ProjectSubscriptionEvent{
		EventType: event.GetEventType(),
		Payload:   env.Payload,
	})
}
