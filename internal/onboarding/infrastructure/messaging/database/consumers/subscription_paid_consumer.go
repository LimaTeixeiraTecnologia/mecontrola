package consumers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type markTokenPaidUseCase interface {
	Execute(ctx context.Context, in input.MarkTokenPaidInput) error
}

type subscriptionActivatedPayload struct {
	SubscriptionID     string    `json:"subscription_id"`
	FunnelToken        string    `json:"funnel_token"`
	PlanCode           string    `json:"plan_code"`
	ExternalSaleID     string    `json:"external_sale_id"`
	CustomerMobileE164 string    `json:"customer_mobile_e164"`
	CustomerEmail      string    `json:"customer_email"`
	PaidAt             time.Time `json:"paid_at"`
	OccurredAt         time.Time `json:"occurred_at"`
}

type SubscriptionPaidConsumer struct {
	usecase markTokenPaidUseCase
	o11y    observability.Observability
}

func NewSubscriptionPaidConsumer(
	uc markTokenPaidUseCase,
	o11y observability.Observability,
) *SubscriptionPaidConsumer {
	return &SubscriptionPaidConsumer{usecase: uc, o11y: o11y}
}

func (c *SubscriptionPaidConsumer) Handle(ctx context.Context, event events.Event) error {
	ctx, span := c.o11y.Tracer().Start(ctx, "onboarding.consumer.subscription_paid.handle")
	defer span.End()

	env, ok := event.GetPayload().(outbox.Envelope)
	if !ok {
		return fmt.Errorf("onboarding.consumer.subscription_paid: unexpected payload type %T", event.GetPayload())
	}

	var p subscriptionActivatedPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return fmt.Errorf("onboarding.consumer.subscription_paid: unmarshal payload: %w", err)
	}

	if p.FunnelToken == "" {
		slog.WarnContext(ctx, "onboarding.consumer.subscription_paid.no_funnel_token",
			"event_id", env.ID,
			"external_sale_id", p.ExternalSaleID,
		)
		return nil
	}

	if err := c.usecase.Execute(ctx, input.MarkTokenPaidInput{
		SubscriptionID:     p.SubscriptionID,
		FunnelToken:        p.FunnelToken,
		CustomerMobileE164: p.CustomerMobileE164,
		CustomerEmail:      p.CustomerEmail,
		ExternalSaleID:     p.ExternalSaleID,
		PaidAt:             p.PaidAt,
	}); err != nil {
		span.RecordError(err)
		return fmt.Errorf("onboarding.consumer.subscription_paid: mark token paid: %w", err)
	}

	return nil
}
