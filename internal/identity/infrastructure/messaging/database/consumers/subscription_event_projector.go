package consumers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

const (
	eventTypeActivated = "billing.subscription.activated"
	eventTypeRenewed   = "billing.subscription.renewed"
	eventTypePastDue   = "billing.subscription.past_due"
	eventTypeCanceled  = "billing.subscription.canceled"
	eventTypeRefunded  = "billing.subscription.refunded"
)

type activatedPayload struct {
	SubscriptionID string    `json:"subscription_id"`
	FunnelToken    string    `json:"funnel_token"`
	PlanCode       string    `json:"plan_code"`
	PeriodStart    time.Time `json:"period_start"`
	PeriodEnd      time.Time `json:"period_end"`
	OccurredAt     time.Time `json:"occurred_at"`
}

type renewedPayload struct {
	SubscriptionID    string    `json:"subscription_id"`
	PlanCode          string    `json:"plan_code"`
	PreviousPeriodEnd time.Time `json:"previous_period_end"`
	PeriodEnd         time.Time `json:"period_end"`
	OccurredAt        time.Time `json:"occurred_at"`
}

type pastDuePayload struct {
	SubscriptionID string    `json:"subscription_id"`
	PeriodEnd      time.Time `json:"period_end"`
	GraceEnd       time.Time `json:"grace_end"`
	OccurredAt     time.Time `json:"occurred_at"`
}

type canceledPayload struct {
	SubscriptionID string    `json:"subscription_id"`
	PeriodEnd      time.Time `json:"period_end"`
	OccurredAt     time.Time `json:"occurred_at"`
}

type refundedPayload struct {
	SubscriptionID string    `json:"subscription_id"`
	OccurredAt     time.Time `json:"occurred_at"`
}

type dbProvider interface {
	DBTX(ctx context.Context) database.DBTX
}

type SubscriptionEventProjector struct {
	factory interfaces.RepositoryFactory
	db      dbProvider
	o11y    observability.Observability
}

func NewSubscriptionEventProjector(
	factory interfaces.RepositoryFactory,
	db dbProvider,
	o11y observability.Observability,
) *SubscriptionEventProjector {
	return &SubscriptionEventProjector{factory: factory, db: db, o11y: o11y}
}

func (p *SubscriptionEventProjector) Handle(ctx context.Context, event events.Event) error {
	ctx, span := p.o11y.Tracer().Start(ctx, "identity.projector.subscription_event.handle")
	defer span.End()

	env, ok := event.GetPayload().(outbox.Envelope)
	if !ok {
		return fmt.Errorf("identity.projector: unexpected payload type %T", event.GetPayload())
	}

	switch event.GetEventType() {
	case eventTypeActivated:
		return p.handleActivated(ctx, env.Payload)
	case eventTypeRenewed:
		return p.handleRenewed(ctx, env.Payload)
	case eventTypePastDue:
		return p.handlePastDue(ctx, env.Payload)
	case eventTypeCanceled:
		return p.handleCanceled(ctx, env.Payload)
	case eventTypeRefunded:
		return p.handleRefunded(ctx, env.Payload)
	default:
		return nil
	}
}

func (p *SubscriptionEventProjector) handleActivated(ctx context.Context, raw json.RawMessage) error {
	var pl activatedPayload
	if err := json.Unmarshal(raw, &pl); err != nil {
		return fmt.Errorf("identity.projector.activated: unmarshal: %w", err)
	}

	repo := p.factory.EntitlementRepository(p.db.DBTX(ctx))

	userID, resolveErr := p.resolveUserID(ctx, pl.SubscriptionID)
	if resolveErr != nil {
		rawBytes, marshalErr := json.Marshal(pl)
		if marshalErr != nil {
			return fmt.Errorf("identity.projector.activated: marshal pending: %w", marshalErr)
		}
		if upsertErr := repo.UpsertPending(ctx, pl.SubscriptionID, pl.FunnelToken, rawBytes); upsertErr != nil {
			return fmt.Errorf("identity.projector.activated: upsert pending: %w", upsertErr)
		}
		p.o11y.Logger().Info(ctx, "identity.entitlement.pending",
			observability.String("subscription_id", pl.SubscriptionID),
		)
		return nil
	}

	record := interfaces.EntitlementRecord{
		UserID:         userID,
		SubscriptionID: pl.SubscriptionID,
		Status:         "ACTIVE",
		PeriodEnd:      pl.PeriodEnd,
	}
	if err := repo.Upsert(ctx, record); err != nil {
		return fmt.Errorf("identity.projector.activated: upsert: %w", err)
	}

	p.o11y.Logger().Info(ctx, "identity.entitlement.projected",
		observability.String("user_id", userID),
		observability.String("subscription_id", pl.SubscriptionID),
		observability.String("status", "ACTIVE"),
	)
	return nil
}

func (p *SubscriptionEventProjector) handleRenewed(ctx context.Context, raw json.RawMessage) error {
	var pl renewedPayload
	if err := json.Unmarshal(raw, &pl); err != nil {
		return fmt.Errorf("identity.projector.renewed: unmarshal: %w", err)
	}
	return p.upsertIfKnown(ctx, pl.SubscriptionID, "ACTIVE", pl.PeriodEnd, time.Time{})
}

func (p *SubscriptionEventProjector) handlePastDue(ctx context.Context, raw json.RawMessage) error {
	var pl pastDuePayload
	if err := json.Unmarshal(raw, &pl); err != nil {
		return fmt.Errorf("identity.projector.past_due: unmarshal: %w", err)
	}
	return p.upsertIfKnown(ctx, pl.SubscriptionID, "PAST_DUE", pl.PeriodEnd, pl.GraceEnd)
}

func (p *SubscriptionEventProjector) handleCanceled(ctx context.Context, raw json.RawMessage) error {
	var pl canceledPayload
	if err := json.Unmarshal(raw, &pl); err != nil {
		return fmt.Errorf("identity.projector.canceled: unmarshal: %w", err)
	}
	return p.upsertIfKnown(ctx, pl.SubscriptionID, "CANCELED_PENDING", pl.PeriodEnd, time.Time{})
}

func (p *SubscriptionEventProjector) handleRefunded(ctx context.Context, raw json.RawMessage) error {
	var pl refundedPayload
	if err := json.Unmarshal(raw, &pl); err != nil {
		return fmt.Errorf("identity.projector.refunded: unmarshal: %w", err)
	}
	return p.upsertIfKnown(ctx, pl.SubscriptionID, "REFUNDED", time.Time{}, time.Time{})
}

func (p *SubscriptionEventProjector) upsertIfKnown(ctx context.Context, subscriptionID string, status string, periodEnd time.Time, graceEnd time.Time) error {
	userID, err := p.resolveUserID(ctx, subscriptionID)
	if err != nil {
		p.o11y.Logger().Info(ctx, "identity.entitlement.pending",
			observability.String("subscription_id", subscriptionID),
		)
		return nil
	}

	repo := p.factory.EntitlementRepository(p.db.DBTX(ctx))
	record := interfaces.EntitlementRecord{
		UserID:         userID,
		SubscriptionID: subscriptionID,
		Status:         status,
		PeriodEnd:      periodEnd,
		GraceEnd:       graceEnd,
	}
	if err := repo.Upsert(ctx, record); err != nil {
		return fmt.Errorf("identity.projector.upsert: %w", err)
	}

	p.o11y.Logger().Info(ctx, "identity.entitlement.projected",
		observability.String("user_id", userID),
		observability.String("subscription_id", subscriptionID),
		observability.String("status", status),
	)
	return nil
}

func (p *SubscriptionEventProjector) resolveUserID(ctx context.Context, subscriptionID string) (string, error) {
	const query = `SELECT user_id FROM billing_subscriptions WHERE id = $1 AND user_id IS NOT NULL LIMIT 1`
	var userID string
	err := p.db.DBTX(ctx).QueryRowContext(ctx, query, subscriptionID).Scan(&userID)
	if err != nil {
		return "", fmt.Errorf("identity.projector.resolve_user_id: %w", err)
	}
	return userID, nil
}
