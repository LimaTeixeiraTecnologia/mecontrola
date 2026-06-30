package consumers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type sendActivationEmailUseCase interface {
	Execute(ctx context.Context, in usecases.SendActivationEmailInput) error
}

type ActivationEmailConsumer struct {
	usecase sendActivationEmailUseCase
	o11y    observability.Observability
	skipped observability.Counter
}

func NewActivationEmailConsumer(
	uc sendActivationEmailUseCase,
	o11y observability.Observability,
) *ActivationEmailConsumer {
	skipped := o11y.Metrics().Counter(
		"onboarding_activation_email_skipped_total",
		"Total de consumos do evento subscription.activated que nao geraram email",
		"1",
	)
	return &ActivationEmailConsumer{usecase: uc, o11y: o11y, skipped: skipped}
}

func (c *ActivationEmailConsumer) Handle(ctx context.Context, event events.Event) error {
	ctx, span := c.o11y.Tracer().Start(ctx, "onboarding.consumer.activation_email.handle")
	defer span.End()

	env, ok := event.GetPayload().(outbox.Envelope)
	if !ok {
		return fmt.Errorf("onboarding.consumer.activation_email: unexpected payload type %T", event.GetPayload())
	}

	var p subscriptionActivatedPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return fmt.Errorf("onboarding.consumer.activation_email: unmarshal payload: %w", err)
	}

	funnelToken := strings.TrimSpace(p.FunnelToken)
	customerEmail := strings.TrimSpace(p.CustomerEmail)
	if funnelToken == "" {
		c.skipped.Add(ctx, 1, observability.String("reason", "missing_funnel_token"))
		slog.WarnContext(ctx, "onboarding.consumer.activation_email.no_funnel_token",
			"event_id", env.ID,
			"external_sale_id", p.ExternalSaleID,
		)
		return nil
	}
	if customerEmail == "" {
		c.skipped.Add(ctx, 1, observability.String("reason", "missing_email"))
		slog.WarnContext(ctx, "onboarding.consumer.activation_email.no_email",
			"event_id", env.ID,
			"external_sale_id", p.ExternalSaleID,
		)
		return nil
	}

	if err := c.usecase.Execute(ctx, usecases.SendActivationEmailInput{
		ClearToken:     funnelToken,
		CustomerEmail:  customerEmail,
		SubscriptionID: p.SubscriptionID,
	}); err != nil {
		span.RecordError(err)
		return fmt.Errorf("onboarding.consumer.activation_email: send: %w", err)
	}

	return nil
}
