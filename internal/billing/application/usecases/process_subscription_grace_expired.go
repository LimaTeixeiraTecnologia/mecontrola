package usecases

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

const graceExpiredBatchDefault = 100

type ProcessSubscriptionGraceExpired struct {
	uow        uow.UnitOfWork[entities.Subscription]
	factory    interfaces.RepositoryFactory
	publisher  interfaces.SubscriptionEventPublisher
	o11y       observability.Observability
	expired    observability.Counter
	batchLimit int
}

func NewProcessSubscriptionGraceExpired(
	u uow.UnitOfWork[entities.Subscription],
	factory interfaces.RepositoryFactory,
	publisher interfaces.SubscriptionEventPublisher,
	o11y observability.Observability,
) *ProcessSubscriptionGraceExpired {
	expired := o11y.Metrics().Counter(
		"billing_subscription_grace_expired_total",
		"Total de subscriptions expiradas apos janela de graca PAST_DUE",
		"1",
	)
	return &ProcessSubscriptionGraceExpired{
		uow:        u,
		factory:    factory,
		publisher:  publisher,
		o11y:       o11y,
		expired:    expired,
		batchLimit: graceExpiredBatchDefault,
	}
}

func (uc *ProcessSubscriptionGraceExpired) Execute(ctx context.Context) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "billing.usecase.process_subscription_grace_expired")
	defer span.End()

	subRepo := uc.factory.SubscriptionRepository(nil)
	now := time.Now().UTC()
	candidates, listErr := subRepo.ListPastDueGraceExpired(ctx, now, uc.batchLimit)
	if listErr != nil {
		return fmt.Errorf("billing.usecase.process_subscription_grace_expired: list candidates: %w", listErr)
	}

	if len(candidates) == 0 {
		return nil
	}

	var errs []error
	for _, cand := range candidates {
		if applyErr := uc.expireOne(ctx, cand); applyErr != nil {
			uc.o11y.Logger().Error(ctx, "billing.usecase.process_subscription_grace_expired.subscription_failed",
				observability.String("subscription_id", cand.SubscriptionID),
				observability.Error(applyErr),
			)
			errs = append(errs, fmt.Errorf("subscription %s: %w", cand.SubscriptionID, applyErr))
			continue
		}
		uc.expired.Add(ctx, 1)
	}

	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("billing.usecase.process_subscription_grace_expired: %w", err)
	}
	return nil
}

func (uc *ProcessSubscriptionGraceExpired) expireOne(ctx context.Context, cand interfaces.ExpiredGraceCandidate) error {
	occurredAt := time.Now().UTC()
	_, execErr := uc.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (entities.Subscription, error) {
		subRepo := uc.factory.SubscriptionRepository(tx)

		if applyErr := subRepo.ApplyTransition(ctx, cand.SubscriptionID, valueobjects.StatusExpired, time.Time{}, occurredAt); applyErr != nil {
			return entities.Subscription{}, fmt.Errorf("apply transition: %w", applyErr)
		}

		expired := entities.HydrateWithUser(
			cand.SubscriptionID,
			cand.UserID,
			valueobjects.FunnelToken{},
			valueobjects.Plan{},
			valueobjects.StatusExpired,
			time.Time{},
			time.Time{},
			time.Time{},
			occurredAt,
		)
		if pubErr := uc.publisher.PublishExpired(ctx, tx, expired, cand.SubscriptionID, cand.GraceEnd); pubErr != nil {
			return entities.Subscription{}, fmt.Errorf("publish expired: %w", pubErr)
		}
		return expired, nil
	})
	return execErr
}
