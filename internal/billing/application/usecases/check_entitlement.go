package usecases

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	identityservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/observability"
)

const entitlementCacheTTL = 5 * time.Minute

// CheckEntitlementUseCase determina se um usuário possui entitlement ativo.
// Cache-first: consulta cache LRU antes de acessar o Postgres (RF-32, RF-33).
type CheckEntitlementUseCase struct {
	subRepo interfaces.SubscriptionRepository
	cache   interfaces.EntitlementCache
	checker identityservices.EntitlementChecker
	o11y    observability.Observability
	metrics *observability.UsecaseMetrics
}

func NewCheckEntitlementUseCase(
	subRepo interfaces.SubscriptionRepository,
	cache interfaces.EntitlementCache,
	o11y observability.Observability,
	metrics *observability.UsecaseMetrics,
) *CheckEntitlementUseCase {
	return &CheckEntitlementUseCase{
		subRepo: subRepo,
		cache:   cache,
		checker: identityservices.NewEntitlementChecker(),
		o11y:    o11y,
		metrics: metrics,
	}
}

func (u *CheckEntitlementUseCase) Execute(ctx context.Context, in input.CheckEntitlementInput) (output.EntitlementDecision, error) {
	return observability.Observe(ctx, u.o11y, u.metrics, "billing", "check_entitlement", func(ctx context.Context) (output.EntitlementDecision, error) {
		return u.execute(ctx, in)
	})
}

func (u *CheckEntitlementUseCase) execute(ctx context.Context, in input.CheckEntitlementInput) (output.EntitlementDecision, error) {
	if cached, ok := u.cache.Get(in.UserID); ok {
		return cached, nil
	}

	now := time.Now().UTC()
	sub, err := u.subRepo.FindActiveByUserID(ctx, in.UserID)
	if err != nil && !isNotFoundError(err) {
		return output.EntitlementDecision{}, fmt.Errorf("check entitlement: buscar subscription: %w", err)
	}

	if sub == nil {
		decision := output.EntitlementDecision{
			Status: "denied",
			Reason: "no_active_subscription",
		}
		u.cache.Set(in.UserID, decision, entitlementCacheTTL)
		return decision, nil
	}

	entitled := u.checker.IsEntitled(sub, now)
	decision := buildDecision(sub, entitled, now)
	u.cache.Set(in.UserID, decision, ttlForDecision(decision, now))
	return decision, nil
}

func buildDecision(sub interface {
	Status() identityservices.SubscriptionStatus
	CurrentPeriodEnd() time.Time
	GracePeriodEnd() time.Time
	PeriodEnd() time.Time
}, entitled bool, now time.Time) output.EntitlementDecision {
	_ = now
	if !entitled {
		return output.EntitlementDecision{
			Status: "denied",
			Reason: "subscription_not_active",
		}
	}
	switch sub.Status() {
	case identityservices.StatusPastDue, identityservices.StatusCanceledPending:
		return output.EntitlementDecision{
			Status:    "grace",
			Reason:    "grace_period",
			ExpiresAt: sub.GracePeriodEnd(),
		}
	default:
		return output.EntitlementDecision{
			Status:    "granted",
			Reason:    "active_subscription",
			ExpiresAt: sub.CurrentPeriodEnd(),
		}
	}
}

func isNotFoundError(err error) bool {
	return errors.Is(err, interfaces.ErrSubscriptionNotFound)
}

func ttlForDecision(decision output.EntitlementDecision, now time.Time) time.Duration {
	if decision.ExpiresAt.IsZero() {
		return entitlementCacheTTL
	}
	remaining := decision.ExpiresAt.Sub(now)
	if remaining <= 0 {
		return 0
	}
	return min(remaining, entitlementCacheTTL)
}
