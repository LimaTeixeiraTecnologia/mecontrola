package dispatcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/payload"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/ratelimit"
)

var ativarRegex = regexp.MustCompile(`(?i)^\s*ATIVAR\s+([A-Za-z0-9_\-]{40,45})\s*$`)

type RouteOutcome string

const (
	OutcomeOnboarding  RouteOutcome = "onboarding"
	OutcomeAgent       RouteOutcome = "agent"
	OutcomeFallback    RouteOutcome = "fallback"
	OutcomeRateLimited RouteOutcome = "rate_limited"
	OutcomeDuplicate   RouteOutcome = "duplicate"
	OutcomeInvalid     RouteOutcome = "invalid"
	OutcomeStaleTS     RouteOutcome = "stale_webhook"
)

const timestampWindow = 5 * time.Minute

type EstablishPrincipalUseCase interface {
	Execute(ctx context.Context, in input.EstablishPrincipalInput) (auth.Principal, error)
}

type DedupRepository interface {
	InsertIfAbsent(ctx context.Context, wamid string) (bool, error)
}

type Dispatcher struct {
	dedup           DedupRepository
	establish       EstablishPrincipalUseCase
	limiter         *ratelimit.Limiter
	publisher       outbox.Publisher
	onboardingRoute func(ctx context.Context, msg payload.Message) RouteOutcome
	agentRoute      func(ctx context.Context, msg payload.Message) RouteOutcome
	o11y            observability.Observability
	routeTotal      observability.Counter
	rateLimitHits   observability.Counter
	staleWebhook    observability.Counter
}

func New(
	dedupRepo DedupRepository,
	establish EstablishPrincipalUseCase,
	limiter *ratelimit.Limiter,
	publisher outbox.Publisher,
	onboardingRoute func(ctx context.Context, msg payload.Message) RouteOutcome,
	agentRoute func(ctx context.Context, msg payload.Message) RouteOutcome,
	o11y observability.Observability,
) *Dispatcher {
	routeTotal := o11y.Metrics().Counter(
		"whatsapp_dispatcher_route_total",
		"Total de mensagens roteadas pelo dispatcher de WhatsApp",
		"1",
	)
	rateLimitHits := o11y.Metrics().Counter(
		"auth_rate_limit_hits_total",
		"Total de mensagens bloqueadas pelo rate limiter por user",
		"1",
	)
	staleWebhook := o11y.Metrics().Counter(
		"whatsapp_stale_webhook_total",
		"Total de webhooks WhatsApp fora da janela de timestamp ou com timestamp invalido",
		"1",
	)
	return &Dispatcher{
		dedup:           dedupRepo,
		establish:       establish,
		limiter:         limiter,
		publisher:       publisher,
		onboardingRoute: onboardingRoute,
		agentRoute:      agentRoute,
		o11y:            o11y,
		routeTotal:      routeTotal,
		rateLimitHits:   rateLimitHits,
		staleWebhook:    staleWebhook,
	}
}

func (d *Dispatcher) finish(ctx context.Context, span observability.Span, outcome RouteOutcome, isActivation, isDedup bool) RouteOutcome {
	span.SetAttributes(
		observability.String("outcome", string(outcome)),
		observability.Bool("is_activation", isActivation),
		observability.Bool("is_dedup", isDedup),
	)
	d.routeTotal.Add(ctx, 1, observability.String("outcome", string(outcome)))
	return outcome
}

func (d *Dispatcher) Route(ctx context.Context, raw json.RawMessage) (RouteOutcome, error) {
	ctx, span := d.o11y.Tracer().Start(ctx, "whatsapp.dispatcher.route")
	defer span.End()
	span.SetAttributes(observability.String("outcome", string(OutcomeInvalid)))

	msg, ok, err := payload.ExtractFirstMessage(raw)
	if err != nil || !ok {
		if err != nil {
			if pubErr := d.publishAuthFailed(ctx, "", "invalid_payload"); pubErr != nil {
				d.o11y.Logger().Error(
					ctx,
					"whatsapp.dispatcher.publish_failed",
					observability.String("reason", "invalid_payload"),
					observability.Error(pubErr),
				)
			}
		}
		return d.finish(ctx, span, OutcomeInvalid, false, false), nil
	}

	inserted, dedupErr := d.dedup.InsertIfAbsent(ctx, msg.WAMID)
	if dedupErr != nil {
		d.o11y.Logger().Error(ctx, "whatsapp.dispatcher.dedup_failed",
			observability.String("wamid", msg.WAMID),
			observability.Error(dedupErr),
		)
		span.RecordError(dedupErr)
		return d.finish(ctx, span, OutcomeInvalid, false, false), fmt.Errorf("whatsapp.dispatcher: dedup insert: %w", dedupErr)
	}
	if !inserted {
		d.o11y.Logger().Info(ctx, "dispatcher.duplicate_wamid", observability.String("wamid", msg.WAMID))
		return d.finish(ctx, span, OutcomeDuplicate, false, true), nil
	}

	if staleReason, stale := d.checkTimestamp(msg.Timestamp); stale {
		d.rejectStale(ctx, staleReason, msg.WAMID)
		return d.finish(ctx, span, OutcomeStaleTS, false, false), nil
	}

	if matches := ativarRegex.FindStringSubmatch(strings.TrimSpace(msg.Text)); matches != nil {
		return d.finish(ctx, span, d.onboardingRoute(ctx, msg), true, false), nil
	}

	principal, establishErr := d.establish.Execute(ctx, input.EstablishPrincipalInput{WhatsAppNumber: msg.From})
	if establishErr != nil {
		if errors.Is(establishErr, application.ErrUnknownUser) {
			return d.finish(ctx, span, d.onboardingRoute(ctx, msg), false, false), nil
		}
		d.o11y.Logger().Error(ctx, "whatsapp.dispatcher.establish_failed",
			observability.String("wa_id_masked", payload.MaskMobile(msg.From)),
			observability.Error(establishErr),
		)
		span.RecordError(establishErr)
		return d.finish(ctx, span, OutcomeInvalid, false, false), fmt.Errorf("whatsapp.dispatcher: establish principal: %w", establishErr)
	}

	if principal.IsZero() {
		return d.finish(ctx, span, d.onboardingRoute(ctx, msg), false, false), nil
	}

	if !d.limiter.Allow(principal.UserID) {
		if pubErr := d.publishAuthFailed(ctx, principal.UserID.String(), "rate_limited"); pubErr != nil {
			d.o11y.Logger().Error(
				ctx,
				"whatsapp.dispatcher.publish_failed",
				observability.String("reason", "rate_limited"),
				observability.Error(pubErr),
			)
		}
		d.rateLimitHits.Add(ctx, 1)
		return d.finish(ctx, span, OutcomeRateLimited, false, false), nil
	}

	ctx = auth.WithPrincipal(ctx, principal)
	return d.finish(ctx, span, d.agentRoute(ctx, msg), false, false), nil
}

func (d *Dispatcher) checkTimestamp(raw string) (string, bool) {
	ts, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return "invalid_webhook_timestamp", true
	}
	delta := time.Now().UTC().Sub(time.Unix(ts, 0).UTC())
	if delta < 0 {
		delta = -delta
	}
	if delta > timestampWindow {
		return "stale_webhook", true
	}
	return "", false
}

func (d *Dispatcher) rejectStale(ctx context.Context, reason, wamid string) {
	d.staleWebhook.Add(ctx, 1, observability.String("reason", reason))
	d.o11y.Logger().Warn(ctx, "dispatcher.stale_webhook",
		observability.String("reason", reason),
		observability.String("wamid", wamid),
	)
	if pubErr := d.publishAuthFailed(ctx, "", reason); pubErr != nil {
		d.o11y.Logger().Error(ctx, "whatsapp.dispatcher.publish_failed",
			observability.String("reason", reason),
			observability.Error(pubErr),
		)
	}
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
		return fmt.Errorf("whatsapp.dispatcher: generate auth_failed event id: %w", err)
	}
	eventID := id.String()
	now := time.Now().UTC()

	p := authFailedPayload{
		EventID:    eventID,
		Kind:       "failed",
		Source:     "whatsapp",
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
		return fmt.Errorf("whatsapp.dispatcher: marshal auth_failed payload: %w", err)
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
