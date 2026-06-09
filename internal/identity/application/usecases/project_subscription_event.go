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
	eventTypeActivated         = "billing.subscription.activated"
	eventTypeRenewed           = "billing.subscription.renewed"
	eventTypePastDue           = "billing.subscription.past_due"
	eventTypeCanceled          = "billing.subscription.canceled"
	eventTypeRefunded          = "billing.subscription.refunded"
	eventTypeSubscriptionBound = "onboarding.subscription_bound"
)

type activatedPayload struct {
	SubscriptionID string `json:"subscription_id"`
}

type renewedPayload struct {
	SubscriptionID string `json:"subscription_id"`
}

type pastDuePayload struct {
	SubscriptionID string `json:"subscription_id"`
}

type canceledPayload struct {
	SubscriptionID string `json:"subscription_id"`
}

type refundedPayload struct {
	SubscriptionID string `json:"subscription_id"`
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
	case eventTypeActivated:
		return u.projectActivated(ctx, in.Payload)
	case eventTypeRenewed:
		return u.projectRenewed(ctx, in.Payload)
	case eventTypePastDue:
		return u.projectPastDue(ctx, in.Payload)
	case eventTypeCanceled:
		return u.projectCanceled(ctx, in.Payload)
	case eventTypeRefunded:
		return u.projectRefunded(ctx, in.Payload)
	case eventTypeSubscriptionBound:
		return u.projectBound(ctx, in.Payload)
	default:
		return nil
	}
}

func (u *ProjectSubscriptionEvent) projectActivated(ctx context.Context, raw json.RawMessage) error {
	var payload activatedPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("identity.usecase.project_subscription_event activated: unmarshal: %w", err)
	}
	return u.projectCurrent(ctx, payload.SubscriptionID)
}

func (u *ProjectSubscriptionEvent) projectRenewed(ctx context.Context, raw json.RawMessage) error {
	var payload renewedPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("identity.usecase.project_subscription_event renewed: unmarshal: %w", err)
	}
	return u.projectCurrent(ctx, payload.SubscriptionID)
}

func (u *ProjectSubscriptionEvent) projectPastDue(ctx context.Context, raw json.RawMessage) error {
	var payload pastDuePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("identity.usecase.project_subscription_event past_due: unmarshal: %w", err)
	}
	return u.projectCurrent(ctx, payload.SubscriptionID)
}

func (u *ProjectSubscriptionEvent) projectCanceled(ctx context.Context, raw json.RawMessage) error {
	var payload canceledPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("identity.usecase.project_subscription_event canceled: unmarshal: %w", err)
	}
	return u.projectCurrent(ctx, payload.SubscriptionID)
}

func (u *ProjectSubscriptionEvent) projectRefunded(ctx context.Context, raw json.RawMessage) error {
	var payload refundedPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("identity.usecase.project_subscription_event refunded: unmarshal: %w", err)
	}
	return u.projectCurrent(ctx, payload.SubscriptionID)
}

type boundPayload struct {
	SubscriptionID string `json:"subscription_id"`
}

func (u *ProjectSubscriptionEvent) projectBound(ctx context.Context, raw json.RawMessage) error {
	var payload boundPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("identity.usecase.project_subscription_event bound: unmarshal: %w", err)
	}
	if payload.SubscriptionID == "" {
		return nil
	}
	return u.projectCurrent(ctx, payload.SubscriptionID)
}

func (u *ProjectSubscriptionEvent) projectCurrent(ctx context.Context, subscriptionID string) error {
	projection, err := u.reader.FindCurrentBySubscriptionID(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("identity.usecase.project_subscription_event current: %w", err)
	}

	repo := u.factory.EntitlementRepository(u.mgr.DBTX(ctx))
	if projection.UserID == "" {
		raw, marshalErr := json.Marshal(projection)
		if marshalErr != nil {
			return fmt.Errorf("identity.usecase.project_subscription_event pending: marshal: %w", marshalErr)
		}
		if upsertErr := repo.UpsertPending(ctx, projection.SubscriptionID, projection.FunnelToken, raw); upsertErr != nil {
			return fmt.Errorf("identity.usecase.project_subscription_event pending: upsert: %w", upsertErr)
		}
		u.o11y.Logger().Info(ctx, "identity.entitlement.pending",
			observability.String("subscription_id", subscriptionID),
		)
		return nil
	}

	record := interfaces.EntitlementRecord{
		UserID:         projection.UserID,
		SubscriptionID: projection.SubscriptionID,
		Status:         projection.Status,
		PeriodEnd:      projection.PeriodEnd,
		GraceEnd:       projection.GraceEnd,
	}
	if err := repo.Upsert(ctx, record); err != nil {
		return fmt.Errorf("identity.usecase.project_subscription_event upsert: %w", err)
	}

	u.o11y.Logger().Info(ctx, "identity.entitlement.projected",
		observability.String("user_id", projection.UserID),
		observability.String("subscription_id", subscriptionID),
		observability.String("status", projection.Status),
	)
	return nil
}
