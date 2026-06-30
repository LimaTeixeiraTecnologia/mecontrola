package consumers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type activateFromInboundUseCase interface {
	Execute(ctx context.Context, in input.ActivateFromInboundInput) (usecases.ActivateFromInboundResult, error)
}

type activationAttemptedPayload struct {
	PeerE164  string `json:"peer_e164"`
	Text      string `json:"text"`
	MessageID string `json:"message_id"`
}

type ActivationAttemptConsumer struct {
	usecase activateFromInboundUseCase
	o11y    observability.Observability
}

func NewActivationAttemptConsumer(
	uc activateFromInboundUseCase,
	o11y observability.Observability,
) *ActivationAttemptConsumer {
	return &ActivationAttemptConsumer{usecase: uc, o11y: o11y}
}

func (c *ActivationAttemptConsumer) Handle(ctx context.Context, event events.Event) error {
	ctx, span := c.o11y.Tracer().Start(ctx, "onboarding.consumer.activation_attempt.handle")
	defer span.End()

	env, ok := event.GetPayload().(outbox.Envelope)
	if !ok {
		return fmt.Errorf("onboarding.consumer.activation_attempt: unexpected payload type %T", event.GetPayload())
	}

	var p activationAttemptedPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return fmt.Errorf("onboarding.consumer.activation_attempt: unmarshal payload: %w", err)
	}

	if p.PeerE164 == "" {
		slog.WarnContext(ctx, "onboarding.consumer.activation_attempt.missing_peer",
			"event_id", env.ID,
		)
		return nil
	}

	result, err := c.usecase.Execute(ctx, input.ActivateFromInboundInput{
		PeerE164:  p.PeerE164,
		Text:      p.Text,
		MessageID: p.MessageID,
	})
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("onboarding.consumer.activation_attempt: activate: %w", err)
	}

	slog.InfoContext(ctx, "onboarding.consumer.activation_attempt.done",
		"event_id", env.ID,
		"outcome", result.Outcome.String(),
	)

	return nil
}
