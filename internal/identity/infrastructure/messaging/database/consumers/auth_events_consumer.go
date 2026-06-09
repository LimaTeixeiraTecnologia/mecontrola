package consumers

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type projectAuthEventUseCase interface {
	Execute(ctx context.Context, in input.ProjectAuthEvent) error
}

type anonymizeUserAuthEventsUseCase interface {
	Execute(ctx context.Context, in input.AnonymizeUserAuthEvents) error
}

type AuthEventsConsumer struct {
	projectAuthEvent        projectAuthEventUseCase
	anonymizeUserAuthEvents anonymizeUserAuthEventsUseCase
	o11y                    observability.Observability
	decodeFails             observability.Counter
}

func NewAuthEventsConsumer(
	projectAuthEvent projectAuthEventUseCase,
	anonymizeUserAuthEvents anonymizeUserAuthEventsUseCase,
	o11y observability.Observability,
) *AuthEventsConsumer {
	decodeFails := o11y.Metrics().Counter(
		"auth_events_consumer_decode_failed_total",
		"Total de falhas de decode do payload do consumer de auth_events",
		"1",
	)
	return &AuthEventsConsumer{
		projectAuthEvent:        projectAuthEvent,
		anonymizeUserAuthEvents: anonymizeUserAuthEvents,
		o11y:                    o11y,
		decodeFails:             decodeFails,
	}
}

func (c *AuthEventsConsumer) Handle(ctx context.Context, event events.Event) error {
	ctx, span := c.o11y.Tracer().Start(ctx, "identity.consumer.auth_events.handle")
	defer span.End()

	payload := event.GetPayload()
	env, ok := payload.(outbox.Envelope)
	if !ok {
		return fmt.Errorf("identity.consumer.auth_events: unexpected payload type %T", payload)
	}

	eventType := event.GetEventType()
	switch eventType {
	case "auth.principal_established", "auth.failed", "auth.unknown_user":
		if err := c.projectAuthEvent.Execute(ctx, input.ProjectAuthEvent{
			EventType: eventType,
			Payload:   env.Payload,
		}); err != nil {
			c.decodeFails.Add(ctx, 1)
			return err
		}
		return nil
	case "user.deleted":
		if err := c.anonymizeUserAuthEvents.Execute(ctx, input.AnonymizeUserAuthEvents{
			Payload: env.Payload,
		}); err != nil {
			c.decodeFails.Add(ctx, 1)
			return err
		}
		return nil
	default:
		return fmt.Errorf("identity.consumer.auth_events: unhandled event type %q", eventType)
	}
}
