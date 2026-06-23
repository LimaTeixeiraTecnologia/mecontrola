package consumers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	platformevents "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type OnboardingBoundConsumer struct {
	router      whatsAppRouter
	o11y        observability.Observability
	decodeFails observability.Counter
}

func NewOnboardingBoundConsumer(router whatsAppRouter, o11y observability.Observability) *OnboardingBoundConsumer {
	decodeFails := o11y.Metrics().Counter(
		"agent_onboarding_bound_consumer_decode_failed_total",
		"Total de falhas de decode do consumer subscription_bound no agente",
		"1",
	)
	return &OnboardingBoundConsumer{
		router:      router,
		o11y:        o11y,
		decodeFails: decodeFails,
	}
}

func (c *OnboardingBoundConsumer) Handle(ctx context.Context, event platformevents.Event) error {
	ctx, span := c.o11y.Tracer().Start(ctx, "agent.consumer.onboarding_bound.handle")
	defer span.End()

	env, ok := event.GetPayload().(outbox.Envelope)
	if !ok {
		return fmt.Errorf("agent.consumer.onboarding_bound: tipo de payload inesperado %T", event.GetPayload())
	}

	var p onboardingBoundPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		c.decodeFails.Add(ctx, 1)
		return fmt.Errorf("agent.consumer.onboarding_bound: deserializar payload: %w", err)
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		c.decodeFails.Add(ctx, 1)
		return fmt.Errorf("agent.consumer.onboarding_bound: parse user_id: %w", err)
	}

	if p.PeerE164 == "" {
		c.o11y.Logger().Warn(ctx, "agent.consumer.onboarding_bound.missing_peer",
			observability.String("user_id", userID.String()),
		)
		return nil
	}

	c.router.RouteWhatsApp(ctx,
		appservices.Principal{UserID: userID},
		appservices.InboundMessage{
			Text:       appservices.OnboardingWelcomeSignal,
			WhatsAppTo: p.PeerE164,
		},
	)
	return nil
}

type onboardingBoundPayload struct {
	UserID   string `json:"user_id"`
	PeerE164 string `json:"peer_e164"`
}
