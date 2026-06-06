package usecases

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain"
)

var ErrEntitlementNotFound = errors.New("identity: entitlement not found for user")

type entitlementView struct {
	subscriptionID string
	status         domain.SubscriptionStatus
	periodEnd      time.Time
	graceEnd       time.Time
}

func (e entitlementView) Status() domain.SubscriptionStatus { return e.status }
func (e entitlementView) PeriodEnd() time.Time              { return e.periodEnd }
func (e entitlementView) GracePeriodEnd() time.Time         { return e.graceEnd }

type DecideUserEntitlement struct {
	mgr     manager.Manager
	factory interfaces.RepositoryFactory
	o11y    observability.Observability
}

func NewDecideUserEntitlement(
	mgr manager.Manager,
	factory interfaces.RepositoryFactory,
	o11y observability.Observability,
) *DecideUserEntitlement {
	return &DecideUserEntitlement{mgr: mgr, factory: factory, o11y: o11y}
}

func (u *DecideUserEntitlement) Execute(ctx context.Context, userID string) (output.EntitlementDecision, error) {
	ctx, span := u.o11y.Tracer().Start(ctx, "identity.usecase.decide_user_entitlement")
	defer span.End()

	entitlementRepo := u.factory.EntitlementRepository(u.mgr.DBTX(ctx))
	record, err := entitlementRepo.FindByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrEntitlementNotFound) {
			entitled, reason := domain.IsEntitled(nil, time.Now().UTC())
			return output.NewEntitlementDecision(entitled, reason, "", "", time.Time{}, time.Time{}), nil
		}
		span.RecordError(err)
		u.o11y.Logger().Error(ctx, "identity.usecase.decide_user_entitlement.failed",
			observability.String("user_id", userID),
			observability.Error(err),
		)
		return output.EntitlementDecision{}, fmt.Errorf("identity.usecase.decide_user_entitlement: %w", err)
	}

	sub := entitlementView{
		subscriptionID: record.SubscriptionID,
		status:         domain.SubscriptionStatus(record.Status),
		periodEnd:      record.PeriodEnd,
		graceEnd:       record.GraceEnd,
	}

	entitled, reason := domain.IsEntitled(sub, time.Now().UTC())
	return output.NewEntitlementDecision(entitled, reason, record.SubscriptionID, record.Status, record.PeriodEnd, record.GraceEnd), nil
}
