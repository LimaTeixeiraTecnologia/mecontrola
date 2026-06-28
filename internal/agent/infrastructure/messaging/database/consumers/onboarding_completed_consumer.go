package consumers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	platformevents "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type onboardingCompletedUseCase interface {
	Execute(ctx context.Context, in usecases.ConsolidateOnboardingWorkingMemoryInput) error
}

type OnboardingCompletedConsumer struct {
	uc           onboardingCompletedUseCase
	o11y         observability.Observability
	decodeFails  observability.Counter
	processTotal observability.Counter
}

func NewOnboardingCompletedConsumer(
	uc onboardingCompletedUseCase,
	o11y observability.Observability,
) *OnboardingCompletedConsumer {
	return &OnboardingCompletedConsumer{
		uc:   uc,
		o11y: o11y,
		decodeFails: o11y.Metrics().Counter(
			"agent_onboarding_completed_consumer_decode_failed_total",
			"Total de falhas de decode do consumer onboarding_completed no agente",
			"1",
		),
		processTotal: o11y.Metrics().Counter(
			"agent_onboarding_completed_consumer_total",
			"Total de execucoes do consumer onboarding_completed no agente",
			"1",
		),
	}
}

func (c *OnboardingCompletedConsumer) Handle(ctx context.Context, event platformevents.Event) error {
	ctx, span := c.o11y.Tracer().Start(ctx, "agent.consumer.onboarding_completed.handle")
	defer span.End()

	env, ok := event.GetPayload().(outbox.Envelope)
	if !ok {
		return fmt.Errorf("agent.consumer.onboarding_completed: tipo de payload inesperado %T", event.GetPayload())
	}

	eventID, err := uuid.Parse(env.ID)
	if err != nil {
		c.decodeFails.Add(ctx, 1)
		return fmt.Errorf("agent.consumer.onboarding_completed: parse event_id: %w", err)
	}

	var p onboardingCompletedPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		c.decodeFails.Add(ctx, 1)
		return fmt.Errorf("agent.consumer.onboarding_completed: deserializar payload: %w", err)
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		c.decodeFails.Add(ctx, 1)
		return fmt.Errorf("agent.consumer.onboarding_completed: parse user_id: %w", err)
	}

	in := usecases.ConsolidateOnboardingWorkingMemoryInput{
		UserID:     userID,
		EventID:    eventID,
		EventType:  env.EventType,
		OccurredAt: env.OccurredAt,
	}
	if in.OccurredAt.IsZero() {
		in.OccurredAt = time.Now().UTC()
	}

	if handleErr := c.uc.Execute(ctx, in); handleErr != nil {
		c.processTotal.Add(ctx, 1, observability.String("result", "error"))
		return fmt.Errorf("agent.consumer.onboarding_completed: %w", handleErr)
	}
	c.processTotal.Add(ctx, 1, observability.String("result", "success"))
	return nil
}

type onboardingCompletedPayload struct {
	UserID string `json:"UserID"`
}
