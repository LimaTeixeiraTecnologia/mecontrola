package dispatcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/channels"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/telegram/payload"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/ratelimit"
)

type RouteOutcome string

const (
	OutcomeOnboarding  RouteOutcome = "onboarding"
	OutcomeAgent       RouteOutcome = "agent"
	OutcomeFallback    RouteOutcome = "fallback"
	OutcomeRateLimited RouteOutcome = "rate_limited"
	OutcomeDuplicate   RouteOutcome = "duplicate"
	OutcomeInvalid     RouteOutcome = "invalid"
	OutcomeStaleTS     RouteOutcome = "stale_webhook"
	OutcomeRejected    RouteOutcome = "rejected"
)

const sourceTelegram = "telegram"

const timestampWindow = 5 * time.Minute

type ResolvePrincipalUseCase interface {
	Execute(ctx context.Context, in input.ResolvePrincipalByIdentity) (auth.Principal, error)
}

type DedupRepository interface {
	InsertIfAbsent(ctx context.Context, botID, updateID int64) (bool, error)
}

type AgentRoute func(ctx context.Context, msg payload.Message) RouteOutcome

type OnboardingRoute func(ctx context.Context, msg payload.Message) RouteOutcome

type Dispatcher struct {
	botID           int64
	dedup           DedupRepository
	resolve         ResolvePrincipalUseCase
	limiter         *ratelimit.Limiter
	publisher       outbox.Publisher
	onboardingRoute OnboardingRoute
	agentRoute      AgentRoute
	o11y            observability.Observability
	routeTotal      observability.Counter
	rejectionTotal  observability.Counter
	rateLimitHits   observability.Counter
	staleWebhook    observability.Counter
}

func New(
	botID int64,
	dedupRepo DedupRepository,
	resolve ResolvePrincipalUseCase,
	limiter *ratelimit.Limiter,
	publisher outbox.Publisher,
	onboardingRoute OnboardingRoute,
	agentRoute AgentRoute,
	o11y observability.Observability,
) *Dispatcher {
	routeTotal := o11y.Metrics().Counter(
		"telegram_dispatcher_route_total",
		"Total de mensagens roteadas pelo dispatcher do Telegram por outcome",
		"1",
	)
	rejectionTotal := o11y.Metrics().Counter(
		"telegram_payload_rejection_total",
		"Total de updates do Telegram rejeitados na fase de parsing por motivo",
		"1",
	)
	rateLimitHits := o11y.Metrics().Counter(
		"telegram_rate_limit_hits_total",
		"Total de mensagens Telegram bloqueadas pelo rate limiter por user",
		"1",
	)
	staleWebhook := o11y.Metrics().Counter(
		"telegram_stale_webhook_total",
		"Total de webhooks Telegram fora da janela de timestamp",
		"1",
	)
	return &Dispatcher{
		botID:           botID,
		dedup:           dedupRepo,
		resolve:         resolve,
		limiter:         limiter,
		publisher:       publisher,
		onboardingRoute: onboardingRoute,
		agentRoute:      agentRoute,
		o11y:            o11y,
		routeTotal:      routeTotal,
		rejectionTotal:  rejectionTotal,
		rateLimitHits:   rateLimitHits,
		staleWebhook:    staleWebhook,
	}
}

func (d *Dispatcher) finish(ctx context.Context, span observability.Span, outcome RouteOutcome) RouteOutcome {
	span.SetAttributes(observability.String("outcome", string(outcome)))
	d.routeTotal.Add(ctx, 1, observability.String("outcome", string(outcome)))
	return outcome
}

func (d *Dispatcher) Route(ctx context.Context, raw json.RawMessage) (RouteOutcome, error) {
	ctx, span := d.o11y.Tracer().Start(ctx, "telegram.dispatcher.route")
	defer span.End()

	msg, outcome, err := d.parseMessage(ctx, span, raw)
	if msg == nil || outcome != "" || err != nil {
		return d.finish(ctx, span, outcome), err
	}
	if outcome, err = d.ensureAcceptableUpdate(ctx, span, *msg); outcome != "" || err != nil {
		return d.finish(ctx, span, outcome), err
	}
	outcome, err = d.routeAcceptedMessage(ctx, span, *msg)
	return d.finish(ctx, span, outcome), err
}

func (d *Dispatcher) parseMessage(ctx context.Context, span observability.Span, raw json.RawMessage) (*payload.Message, RouteOutcome, error) {
	outcome, err := payload.ExtractFirstMessage(raw)
	if err != nil {
		d.rejectionTotal.Add(ctx, 1, observability.String("reason", outcome.Kind.String()))
		d.logPublishFailure(ctx, "", "invalid_payload")
		span.RecordError(err)
		return nil, OutcomeInvalid, nil
	}
	if outcome.Kind != payload.RejectAccepted {
		d.rejectionTotal.Add(ctx, 1, observability.String("reason", outcome.Kind.String()))
		d.o11y.Logger().Info(ctx, "telegram.dispatcher.rejected",
			observability.String("reason", outcome.Kind.String()),
			observability.Int64("update_id", outcome.UpdateID),
		)
		return nil, OutcomeRejected, nil
	}
	return &outcome.Message, "", nil
}

func (d *Dispatcher) ensureAcceptableUpdate(ctx context.Context, span observability.Span, msg payload.Message) (RouteOutcome, error) {
	inserted, err := d.dedup.InsertIfAbsent(ctx, d.botID, msg.UpdateID)
	if err != nil {
		d.o11y.Logger().Error(ctx, "telegram.dispatcher.dedup_failed",
			observability.Int64("update_id", msg.UpdateID),
			observability.Error(err),
		)
		span.RecordError(err)
		return OutcomeInvalid, fmt.Errorf("telegram.dispatcher: dedup insert: %w", err)
	}
	if !inserted {
		d.o11y.Logger().Info(ctx, "telegram.dispatcher.duplicate_update",
			observability.Int64("update_id", msg.UpdateID),
		)
		return OutcomeDuplicate, nil
	}
	if d.isStale(msg.UnixDate) {
		d.rejectStale(ctx, msg.UpdateID)
		return OutcomeStaleTS, nil
	}
	return "", nil
}

func (d *Dispatcher) routeAcceptedMessage(ctx context.Context, span observability.Span, msg payload.Message) (RouteOutcome, error) {
	if _, ok := channels.MatchActivationCommand(msg.Text); ok {
		return d.onboardingRoute(ctx, msg), nil
	}
	principal, err := d.resolvePrincipal(ctx, span, msg)
	if err != nil {
		return OutcomeInvalid, err
	}
	if principal.IsZero() {
		return d.onboardingRoute(ctx, msg), nil
	}
	if !d.limiter.Allow(principal.UserID) {
		d.logPublishFailure(ctx, principal.UserID.String(), "rate_limited")
		d.rateLimitHits.Add(ctx, 1)
		return OutcomeRateLimited, nil
	}
	return d.agentRoute(auth.WithPrincipal(ctx, principal), msg), nil
}

func (d *Dispatcher) resolvePrincipal(ctx context.Context, span observability.Span, msg payload.Message) (auth.Principal, error) {
	principal, err := d.resolve.Execute(ctx, input.ResolvePrincipalByIdentity{
		Channel:    string(auth.SourceTelegram),
		ExternalID: msg.ExternalID(),
	})
	if err == nil {
		return principal, nil
	}
	if errors.Is(err, application.ErrUnknownUser) {
		return auth.Principal{}, nil
	}
	d.o11y.Logger().Error(ctx, "telegram.dispatcher.resolve_failed",
		observability.String("from_user_id_masked", payload.MaskUserID(msg.FromUserID)),
		observability.Error(err),
	)
	span.RecordError(err)
	return auth.Principal{}, fmt.Errorf("telegram.dispatcher: resolve principal: %w", err)
}

func (d *Dispatcher) logPublishFailure(ctx context.Context, userID, reason string) {
	pubErr := d.publishAuthFailed(ctx, userID, reason)
	if pubErr == nil {
		return
	}
	d.o11y.Logger().Error(ctx, "telegram.dispatcher.publish_failed",
		observability.String("reason", reason),
		observability.Error(pubErr),
	)
}

func (d *Dispatcher) isStale(unixDate int64) bool {
	if unixDate <= 0 {
		return true
	}
	delta := time.Now().UTC().Sub(time.Unix(unixDate, 0).UTC())
	if delta < 0 {
		delta = -delta
	}
	return delta > timestampWindow
}

func (d *Dispatcher) rejectStale(ctx context.Context, updateID int64) {
	d.staleWebhook.Add(ctx, 1)
	d.o11y.Logger().Warn(ctx, "telegram.dispatcher.stale_webhook",
		observability.Int64("update_id", updateID),
	)
	d.logPublishFailure(ctx, "", "stale_webhook")
}

type authFailedPayload struct {
	EventID    string  `json:"event_id"`
	UserID     *string `json:"user_id"`
	Kind       string  `json:"kind"`
	Source     string  `json:"source"`
	Reason     *string `json:"reason"`
	OccurredAt string  `json:"occurred_at"`
}

func (d *Dispatcher) publishAuthFailed(ctx context.Context, userID, reason string) error {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("telegram.dispatcher: generate auth_failed event id: %w", err)
	}
	eventID := id.String()
	now := time.Now().UTC()

	p := authFailedPayload{
		EventID:    eventID,
		Kind:       "failed",
		Source:     sourceTelegram,
		OccurredAt: now.Format(time.RFC3339),
	}
	if userID != "" {
		p.UserID = &userID
	}
	if reason != "" {
		p.Reason = &reason
	}

	rawPayload, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("telegram.dispatcher: marshal auth_failed payload: %w", err)
	}

	aggregateID := eventID
	if userID != "" {
		aggregateID = userID
	}

	ev := outbox.Event{
		ID:              eventID,
		Type:            "auth.failed",
		AggregateType:   "auth_event",
		AggregateID:     aggregateID,
		AggregateUserID: userID,
		Payload:         rawPayload,
		OccurredAt:      now,
	}

	return d.publisher.Publish(ctx, ev)
}
