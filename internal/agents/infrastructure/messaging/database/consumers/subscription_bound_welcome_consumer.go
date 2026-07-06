package consumers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	gotel "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/formatting"
)

type welcomeOnboardingStarter interface {
	StartOnboarding(ctx context.Context, userID, peer string) (usecases.OnboardingResult, error)
}

type WelcomeDedupStore interface {
	InsertIfAbsent(ctx context.Context, eventID string) (bool, error)
	Delete(ctx context.Context, eventID string) error
}

type subscriptionBoundWelcomePayload struct {
	EventID  string `json:"event_id"`
	UserID   string `json:"user_id"`
	PeerE164 string `json:"peer_e164"`
}

type SubscriptionBoundWelcomeConsumer struct {
	resolveOnboarding welcomeOnboardingStarter
	dedup             WelcomeDedupStore
	gateway           whatsAppTextSender
	o11y              observability.Observability
	welcomeTotal      observability.Counter
}

func NewSubscriptionBoundWelcomeConsumer(
	resolveOnboarding welcomeOnboardingStarter,
	dedup WelcomeDedupStore,
	gateway whatsAppTextSender,
	o11y observability.Observability,
) *SubscriptionBoundWelcomeConsumer {
	welcomeTotal := o11y.Metrics().Counter(
		"agents_onboarding_welcome_total",
		"Total de welcomes de onboarding disparados a partir da ativacao",
		"1",
	)
	return &SubscriptionBoundWelcomeConsumer{
		resolveOnboarding: resolveOnboarding,
		dedup:             dedup,
		gateway:           gateway,
		o11y:              o11y,
		welcomeTotal:      welcomeTotal,
	}
}

func (c *SubscriptionBoundWelcomeConsumer) Handle(ctx context.Context, event events.Event) error {
	env, ok := event.GetPayload().(outbox.Envelope)
	if !ok {
		return fmt.Errorf("agents.consumer.subscription_bound_welcome: unexpected payload type %T", event.GetPayload())
	}

	ctx = gotel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(env.Metadata))

	ctx, span := c.o11y.Tracer().Start(ctx, "agents.consumer.subscription_bound_welcome.handle")
	defer span.End()

	var p subscriptionBoundWelcomePayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		c.welcomeTotal.Add(ctx, 1, observability.String("outcome", "decode_failed"))
		return fmt.Errorf("agents.consumer.subscription_bound_welcome: deserializar payload: %w", err)
	}

	if p.EventID == "" || p.UserID == "" || p.PeerE164 == "" {
		c.welcomeTotal.Add(ctx, 1, observability.String("outcome", "decode_failed"))
		return fmt.Errorf("agents.consumer.subscription_bound_welcome: payload incompleto: event_id=%q user_id=%q peer=%q", p.EventID, p.UserID, p.PeerE164)
	}

	inserted, err := c.dedup.InsertIfAbsent(ctx, p.EventID)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("agents.consumer.subscription_bound_welcome: dedup: %w", err)
	}
	if !inserted {
		c.welcomeTotal.Add(ctx, 1, observability.String("outcome", "deduplicated"))
		return nil
	}

	result, err := c.resolveOnboarding.StartOnboarding(ctx, p.UserID, p.PeerE164)
	if err != nil {
		c.compensate(ctx, p.EventID)
		span.RecordError(err)
		c.welcomeTotal.Add(ctx, 1, observability.String("outcome", "onboarding_error"))
		return fmt.Errorf("agents.consumer.subscription_bound_welcome: onboarding: %w", err)
	}

	content := formatting.NormalizeOutboundText(result.Message)
	if !result.Handled || strings.TrimSpace(content) == "" {
		c.welcomeTotal.Add(ctx, 1, observability.String("outcome", "skipped"))
		return nil
	}

	if err := c.gateway.SendTextMessage(ctx, p.PeerE164, content); err != nil {
		c.compensate(ctx, p.EventID)
		span.RecordError(err)
		c.welcomeTotal.Add(ctx, 1, observability.String("outcome", "send_error"))
		return fmt.Errorf("agents.consumer.subscription_bound_welcome: enviar welcome: %w", err)
	}

	c.welcomeTotal.Add(ctx, 1, observability.String("outcome", "sent"))
	return nil
}

func (c *SubscriptionBoundWelcomeConsumer) compensate(ctx context.Context, eventID string) {
	if err := c.dedup.Delete(ctx, eventID); err != nil {
		c.o11y.Logger().Error(ctx, "agents.consumer.subscription_bound_welcome: compensar dedup", observability.Error(err))
	}
}
