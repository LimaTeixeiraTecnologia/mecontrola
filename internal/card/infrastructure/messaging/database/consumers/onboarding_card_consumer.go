package consumers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/idempotency"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

const (
	eventIdempotencyScope = "event:onboarding.card_registered"
	eventIdempotencyTTL   = 30 * 24 * time.Hour
)

type cardCreator interface {
	Execute(ctx context.Context, in input.CreateCard) (output.Card, error)
}

type idempotencyStorage interface {
	Get(ctx context.Context, scope, key, userID string) (idempotency.Record, error)
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
	idemStorage idempotencyStorage
	o11y        observability.Observability
	decodeFails observability.Counter
}

func NewOnboardingCardConsumer(createCard cardCreator, idemStorage idempotencyStorage, o11y observability.Observability) *OnboardingCardConsumer {
	decodeFails := o11y.Metrics().Counter(
		"card_onboarding_card_consumer_decode_failed_total",
		"Total de falhas de decode do consumer de onboarding card",
		"1",
	)
	return &OnboardingCardConsumer{
		createCard:  createCard,
		idemStorage: idemStorage,
		o11y:        o11y,
		decodeFails: decodeFails,
	}
}

func (c *OnboardingCardConsumer) Handle(ctx context.Context, event events.Event) error {
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

	alreadyProcessed, err := c.guardIdempotency(ctx, env.ID, userID.String())
	if err != nil {
		return err
	}
	if alreadyProcessed {
		return nil
	}

	if p.DueDay < 1 || p.DueDay > 31 {
		return fmt.Errorf("card.consumer.onboarding_card: due_day %d inválido (1..31)", p.DueDay)
	}

	dueDay := p.DueDay
	hash := sha256.Sum256([]byte(env.ID))
	ctx = idempotency.WithContext(ctx, idempotency.IdempotencyContext{
		Scope:       eventIdempotencyScope,
		Key:         env.ID,
		UserID:      userID.String(),
		RequestHash: hex.EncodeToString(hash[:]),
		ExpiresAt:   time.Now().UTC().Add(eventIdempotencyTTL),
	})

	if _, execErr := c.createCard.Execute(ctx, input.CreateCard{
		UserID:     userID,
		Name:       p.Name,
		Nickname:   p.Name,
		ClosingDay: p.ClosingDay,
		DueDay:     &dueDay,
		LimitCents: p.LimitCents,
	}); execErr != nil {
		return fmt.Errorf("card.consumer.onboarding_card: criar cartão: %w", execErr)
	}

	return nil
}

func (c *OnboardingCardConsumer) guardIdempotency(ctx context.Context, eventID, userID string) (bool, error) {
	if c.idemStorage == nil {
		return false, nil
	}
	_, err := c.idemStorage.Get(ctx, eventIdempotencyScope, eventID, userID)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, idempotency.ErrNotFound) {
		return false, nil
	}
	return false, fmt.Errorf("card.consumer.onboarding_card: idempotency check: %w", err)
}
