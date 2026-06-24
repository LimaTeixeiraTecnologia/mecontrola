package consumers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	platformevents "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type cardCreator interface {
	Execute(ctx context.Context, in input.CreateCard) (output.Card, error)
}

type cardRegisteredPayload struct {
	UserID     string `json:"UserID"`
	Name       string `json:"Name"`
	LimitCents int64  `json:"LimitCents"`
	ClosingDay int    `json:"ClosingDay"`
	DueDay     int    `json:"DueDay"`
}

type OnboardingCardConsumer struct {
	createCard  cardCreator
	o11y        observability.Observability
	decodeFails observability.Counter
}

func NewOnboardingCardConsumer(createCard cardCreator, o11y observability.Observability) *OnboardingCardConsumer {
	decodeFails := o11y.Metrics().Counter(
		"card_onboarding_card_consumer_decode_failed_total",
		"Total de falhas de decode do consumer de onboarding card",
		"1",
	)
	return &OnboardingCardConsumer{
		createCard:  createCard,
		o11y:        o11y,
		decodeFails: decodeFails,
	}
}

func (c *OnboardingCardConsumer) Handle(ctx context.Context, event platformevents.Event) error {
	ctx, span := c.o11y.Tracer().Start(ctx, "card.consumer.onboarding_card.handle")
	defer span.End()

	rawPayload := event.GetPayload()
	env, ok := rawPayload.(outbox.Envelope)
	if !ok {
		return fmt.Errorf("card.consumer.onboarding_card: tipo de payload inesperado %T", rawPayload)
	}

	var p cardRegisteredPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		c.decodeFails.Add(ctx, 1)
		return fmt.Errorf("card.consumer.onboarding_card: deserializar payload: %w", err)
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		c.decodeFails.Add(ctx, 1)
		return fmt.Errorf("card.consumer.onboarding_card: user_id inválido: %w", err)
	}

	var dueDay *int
	if p.DueDay > 0 {
		d := p.DueDay
		dueDay = &d
	}
	if _, execErr := c.createCard.Execute(ctx, input.CreateCard{
		UserID:     userID,
		Name:       p.Name,
		Nickname:   p.Name,
		ClosingDay: p.ClosingDay,
		DueDay:     dueDay,
		LimitCents: p.LimitCents,
	}); execErr != nil {
		return fmt.Errorf("card.consumer.onboarding_card: criar cartão: %w", execErr)
	}

	return nil
}
