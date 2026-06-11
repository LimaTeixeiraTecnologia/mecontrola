package usecases

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
)

const (
	prefixProjectSubscriptionEvent = "identity.usecase.project_subscription_event:"
	eventTypeActivated             = "billing.subscription.activated"
	eventTypeRenewed               = "billing.subscription.renewed"
	eventTypePastDue               = "billing.subscription.past_due"
	eventTypeCanceled              = "billing.subscription.canceled"
	eventTypeRefunded              = "billing.subscription.refunded"
	eventTypeSubscriptionBound     = "onboarding.subscription_bound"
)

type subscriptionRefPayload struct {
	SubscriptionID string `json:"subscription_id"`
}

type entitlementPlan struct {
	record     interfaces.EntitlementRecord
	pendingRaw []byte
	isPending  bool
}

type ProjectSubscriptionEvent struct {
	factory interfaces.RepositoryFactory
	mgr     manager.Manager
	reader  interfaces.SubscriptionProjectionReader
	o11y    observability.Observability
}

func NewProjectSubscriptionEvent(
	factory interfaces.RepositoryFactory,
	mgr manager.Manager,
	reader interfaces.SubscriptionProjectionReader,
	o11y observability.Observability,
) *ProjectSubscriptionEvent {
	return &ProjectSubscriptionEvent{factory: factory, mgr: mgr, reader: reader, o11y: o11y}
}

func (u *ProjectSubscriptionEvent) Execute(ctx context.Context, in input.ProjectSubscriptionEvent) error {
	ctx, span := u.o11y.Tracer().Start(ctx, "identity.usecase.project_subscription_event")
	defer span.End()

	switch in.EventType {
	case eventTypeActivated, eventTypeRenewed, eventTypePastDue, eventTypeCanceled, eventTypeRefunded:
		subscriptionID, err := extractSubscriptionRef(in.Payload, in.EventType)
		if err != nil {
			return err
		}
		return u.projectCurrent(ctx, subscriptionID)

	case eventTypeSubscriptionBound:
		subscriptionID, err := extractSubscriptionRef(in.Payload, in.EventType)
		if err != nil {
			return err
		}
		if subscriptionID == "" {
			return nil
		}
		return u.projectCurrent(ctx, subscriptionID)

	default:
		return nil
	}
}

func extractSubscriptionRef(raw json.RawMessage, eventType string) (string, error) {
	var payload subscriptionRefPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", fmt.Errorf("%s %s: unmarshal: %w", prefixProjectSubscriptionEvent, eventType, err)
	}
	return payload.SubscriptionID, nil
}

func planEntitlementUpsert(projection interfaces.SubscriptionProjectionRecord) (entitlementPlan, error) {
	if projection.UserID == "" {
		raw, err := json.Marshal(projection)
		if err != nil {
			return entitlementPlan{}, fmt.Errorf("marshal pending projection: %w", err)
		}
		return entitlementPlan{pendingRaw: raw, isPending: true}, nil
	}

	return entitlementPlan{
		record: interfaces.EntitlementRecord{
			UserID:         projection.UserID,
			SubscriptionID: projection.SubscriptionID,
			Status:         projection.Status,
			PeriodEnd:      projection.PeriodEnd,
			GraceEnd:       projection.GraceEnd,
		},
	}, nil
}

func (u *ProjectSubscriptionEvent) projectCurrent(ctx context.Context, subscriptionID string) error {
	projection, err := u.reader.FindCurrentBySubscriptionID(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("%s current: %w", prefixProjectSubscriptionEvent, err)
	}

	plan, err := planEntitlementUpsert(projection)
	if err != nil {
		return fmt.Errorf("%s plan: %w", prefixProjectSubscriptionEvent, err)
	}

	repo := u.factory.EntitlementRepository(u.mgr.DBTX(ctx))

	if plan.isPending {
		if err := repo.UpsertPending(ctx, projection.SubscriptionID, projection.FunnelToken, plan.pendingRaw); err != nil {
			return fmt.Errorf("%s pending upsert: %w", prefixProjectSubscriptionEvent, err)
		}
		u.o11y.Logger().Info(ctx, "identity.entitlement.pending",
			observability.String("subscription_id", subscriptionID),
		)
		return nil
	}

	if err := repo.Upsert(ctx, plan.record); err != nil {
		return fmt.Errorf("%s upsert: %w", prefixProjectSubscriptionEvent, err)
	}

	u.o11y.Logger().Info(ctx, "identity.entitlement.projected",
		observability.String("user_id", projection.UserID),
		observability.String("subscription_id", subscriptionID),
		observability.String("status", projection.Status),
	)
	return nil
}
