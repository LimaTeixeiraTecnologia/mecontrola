package consumers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type welcomeGateway interface {
	SendTextMessage(ctx context.Context, toE164, text string) error
}

type welcomeDedup interface {
	InsertIfAbsent(ctx context.Context, eventID string) (bool, error)
}

type subscriptionBoundPayload struct {
	PeerE164 string    `json:"peer_e164"`
	BoundAt  time.Time `json:"bound_at"`
}

type WelcomeConsumer struct {
	gateway          welcomeGateway
	dedup            welcomeDedup
	welcomeMsg       string
	introMsg         string
	activationWindow time.Duration
	o11y             observability.Observability
}

func NewWelcomeConsumer(
	gateway welcomeGateway,
	dedup welcomeDedup,
	welcomeMsg string,
	introMsg string,
	activationWindow time.Duration,
	o11y observability.Observability,
) *WelcomeConsumer {
	return &WelcomeConsumer{
		gateway:          gateway,
		dedup:            dedup,
		welcomeMsg:       welcomeMsg,
		introMsg:         introMsg,
		activationWindow: activationWindow,
		o11y:             o11y,
	}
}

func (c *WelcomeConsumer) Handle(ctx context.Context, event events.Event) error {
	ctx, span := c.o11y.Tracer().Start(ctx, "onboarding.consumer.welcome.handle")
	defer span.End()

	env, ok := event.GetPayload().(outbox.Envelope)
	if !ok {
		return fmt.Errorf("onboarding.consumer.welcome: unexpected payload type %T", event.GetPayload())
	}

	var p subscriptionBoundPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return fmt.Errorf("onboarding.consumer.welcome: unmarshal payload: %w", err)
	}

	if p.PeerE164 == "" {
		slog.WarnContext(ctx, "onboarding.consumer.welcome.missing_peer",
			"event_id", env.ID,
		)
		return nil
	}

	now := time.Now().UTC()
	if c.activationWindow > 0 && now.Sub(p.BoundAt) > c.activationWindow {
		slog.InfoContext(ctx, "onboarding.consumer.welcome.outside_window",
			"event_id", env.ID,
		)
		return nil
	}

	claimed, claimErr := c.dedup.InsertIfAbsent(ctx, env.ID)
	if claimErr != nil {
		span.RecordError(claimErr)
		return fmt.Errorf("onboarding.consumer.welcome: dedup: %w", claimErr)
	}
	if !claimed {
		slog.InfoContext(ctx, "onboarding.consumer.welcome.duplicate",
			"event_id", env.ID,
		)
		return nil
	}

	if err := c.gateway.SendTextMessage(ctx, p.PeerE164, c.welcomeMsg); err != nil {
		span.RecordError(err)
		return fmt.Errorf("onboarding.consumer.welcome: send welcome: %w", err)
	}

	if err := c.gateway.SendTextMessage(ctx, p.PeerE164, c.introMsg); err != nil {
		span.RecordError(err)
		return fmt.Errorf("onboarding.consumer.welcome: send intro: %w", err)
	}

	slog.InfoContext(ctx, "onboarding.consumer.welcome.sent",
		"event_id", env.ID,
	)

	return nil
}
