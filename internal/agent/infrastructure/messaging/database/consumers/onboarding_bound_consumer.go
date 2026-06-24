package consumers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	appusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	platformevents "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type onboardingInProgressChecker interface {
	Load(ctx context.Context, userID uuid.UUID) (appusecases.OnboardingSnapshot, error)
}

type greetingDecisionStore interface {
	FindByMessageID(ctx context.Context, userID uuid.UUID, channel, messageID string) (bool, error)
	RegisterGreeting(ctx context.Context, userID uuid.UUID, channel, messageID string, now time.Time) error
}

type greetingWelcomeMarker interface {
	MarkWelcomeSent(ctx context.Context, userID uuid.UUID) (alreadySent bool, err error)
}

type OnboardingBoundConsumer struct {
	router        whatsAppRouter
	o11y          observability.Observability
	decodeFails   observability.Counter
	dedupTotal    observability.Counter
	stateChecker  onboardingInProgressChecker
	decisionStore greetingDecisionStore
	welcomeMarker greetingWelcomeMarker
}

type OnboardingBoundConsumerOption func(*OnboardingBoundConsumer)

func WithOnboardingStateChecker(checker onboardingInProgressChecker) OnboardingBoundConsumerOption {
	return func(c *OnboardingBoundConsumer) {
		c.stateChecker = checker
	}
}

func WithGreetingDecisionStore(store greetingDecisionStore) OnboardingBoundConsumerOption {
	return func(c *OnboardingBoundConsumer) {
		c.decisionStore = store
	}
}

func WithGreetingWelcomeMarker(marker greetingWelcomeMarker) OnboardingBoundConsumerOption {
	return func(c *OnboardingBoundConsumer) {
		c.welcomeMarker = marker
	}
}

func NewOnboardingBoundConsumer(router whatsAppRouter, o11y observability.Observability, opts ...OnboardingBoundConsumerOption) *OnboardingBoundConsumer {
	c := &OnboardingBoundConsumer{
		router: router,
		o11y:   o11y,
		decodeFails: o11y.Metrics().Counter(
			"agent_onboarding_bound_consumer_decode_failed_total",
			"Total de falhas de decode do consumer subscription_bound no agente",
			"1",
		),
		dedupTotal: o11y.Metrics().Counter(
			"agent_onboarding_welcome_dedup_total",
			"Total de deduplicacoes da saudacao de onboarding",
			"1",
		),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
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

	messageID := env.ID
	channel := appservices.ChannelWhatsApp

	skip, err := c.shouldSkipGreeting(ctx, userID, channel, messageID)
	if err != nil {
		return err
	}
	if skip {
		return nil
	}

	res := c.router.RouteWhatsApp(ctx,
		appservices.Principal{UserID: userID},
		appservices.InboundMessage{
			Text:       appservices.OnboardingWelcomeSignal,
			WhatsAppTo: p.PeerE164,
			MessageID:  messageID,
		},
	)

	if !res.Delivered {
		c.o11y.Logger().Warn(ctx, "agent.consumer.onboarding_bound.greeting_not_delivered",
			observability.String("user_id", userID.String()),
			observability.String("outcome", res.Outcome.String()),
		)
		return fmt.Errorf("agent.consumer.onboarding_bound: saudação não entregue (retry): outcome=%s", res.Outcome.String())
	}

	return c.persistGreetingIdempotency(ctx, userID, channel, messageID)
}

func (c *OnboardingBoundConsumer) shouldSkipGreeting(ctx context.Context, userID uuid.UUID, channel, messageID string) (bool, error) {
	if c.decisionStore != nil {
		alreadyDecided, findErr := c.decisionStore.FindByMessageID(ctx, userID, channel, messageID)
		if findErr != nil {
			c.o11y.Logger().Warn(ctx, "agent.consumer.onboarding_bound.decision_lookup_failed",
				observability.String("user_id", userID.String()),
				observability.Error(findErr),
			)
		} else if alreadyDecided {
			c.dedupTotal.Add(ctx, 1, observability.String("result", "skipped"))
			return true, nil
		}
	}

	if c.stateChecker != nil {
		snapshot, loadErr := c.stateChecker.Load(ctx, userID)
		if loadErr != nil {
			return false, fmt.Errorf("agent.consumer.onboarding_bound: verificar estado da sessao: %w", loadErr)
		}
		if !snapshot.InProgress {
			c.o11y.Logger().Warn(ctx, "agent.consumer.onboarding_bound.onboarding_not_started",
				observability.String("user_id", userID.String()),
			)
			return false, fmt.Errorf("agent.consumer.onboarding_bound: sessao de onboarding ausente ou inativa (retry)")
		}
		if snapshot.WelcomeSent {
			c.dedupTotal.Add(ctx, 1, observability.String("result", "skipped"))
			return true, nil
		}
	}

	return false, nil
}

func (c *OnboardingBoundConsumer) persistGreetingIdempotency(ctx context.Context, userID uuid.UUID, channel, messageID string) error {
	now := time.Now().UTC()

	var registered, marked bool
	var idempotencyErr error

	if c.decisionStore != nil {
		if regErr := c.decisionStore.RegisterGreeting(ctx, userID, channel, messageID, now); regErr != nil {
			c.o11y.Logger().Warn(ctx, "agent.consumer.onboarding_bound.register_decision_failed",
				observability.String("user_id", userID.String()),
				observability.Error(regErr),
			)
			idempotencyErr = errors.Join(idempotencyErr, regErr)
		} else {
			registered = true
		}
	}

	if c.welcomeMarker != nil {
		if _, markErr := c.welcomeMarker.MarkWelcomeSent(ctx, userID); markErr != nil {
			c.o11y.Logger().Warn(ctx, "agent.consumer.onboarding_bound.mark_welcome_failed",
				observability.String("user_id", userID.String()),
				observability.Error(markErr),
			)
			idempotencyErr = errors.Join(idempotencyErr, markErr)
		} else {
			marked = true
			c.dedupTotal.Add(ctx, 1, observability.String("result", "sent"))
		}
	}

	if !registered && !marked && idempotencyErr != nil {
		return fmt.Errorf("agent.consumer.onboarding_bound: persistir idempotência da saudação (retry): %w", idempotencyErr)
	}

	return nil
}

type onboardingBoundPayload struct {
	UserID   string `json:"user_id"`
	PeerE164 string `json:"peer_e164"`
}
