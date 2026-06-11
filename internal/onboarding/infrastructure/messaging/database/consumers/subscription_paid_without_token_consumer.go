package consumers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type handlePaidWithoutTokenUseCase interface {
	Execute(ctx context.Context, in input.HandlePaidWithoutTokenInput) error
}

type subscriptionActivatedWithoutTokenPayload struct {
	SubscriptionID     string    `json:"subscription_id"`
	PlanCode           string    `json:"plan_code"`
	ExternalSaleID     string    `json:"external_sale_id"`
	CustomerMobileE164 string    `json:"customer_mobile_e164"`
	CustomerEmail      string    `json:"customer_email"`
	PaidAt             time.Time `json:"paid_at"`
	OccurredAt         time.Time `json:"occurred_at"`
}

type PaidWithoutTokenConsumer struct {
	usecase handlePaidWithoutTokenUseCase
	o11y    observability.Observability
}

func NewPaidWithoutTokenConsumer(
	uc handlePaidWithoutTokenUseCase,
	o11y observability.Observability,
) *PaidWithoutTokenConsumer {
	return &PaidWithoutTokenConsumer{usecase: uc, o11y: o11y}
}

func (c *PaidWithoutTokenConsumer) Handle(ctx context.Context, event events.Event) error {
	ctx, span := c.o11y.Tracer().Start(ctx, "onboarding.consumer.paid_without_token.handle")
	defer span.End()

	evt, ok := event.GetPayload().(outbox.Envelope)
	if !ok {
		return fmt.Errorf("onboarding.consumer.paid_without_token: unexpected payload type %T", event.GetPayload())
	}

	var p subscriptionActivatedWithoutTokenPayload
	if err := json.Unmarshal(evt.Payload, &p); err != nil {
		return fmt.Errorf("onboarding.consumer.paid_without_token: unmarshal payload: %w", err)
	}

	if err := c.usecase.Execute(ctx, input.HandlePaidWithoutTokenInput{
		ExternalSaleID:     p.ExternalSaleID,
		CustomerMobileE164: p.CustomerMobileE164,
		CustomerEmail:      p.CustomerEmail,
		PaidAt:             p.PaidAt,
	}); err != nil {
		span.RecordError(err)
		return fmt.Errorf("onboarding.consumer.paid_without_token: handle: %w", err)
	}

	return nil
}
