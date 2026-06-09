package usecases

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

type ProcessSubscriptionCanceled struct {
	uow       uow.UnitOfWork[entities.Subscription]
	factory   interfaces.RepositoryFactory
	publisher interfaces.SubscriptionEventPublisher
	o11y      observability.Observability
}

func NewProcessSubscriptionCanceled(
	u uow.UnitOfWork[entities.Subscription],
	factory interfaces.RepositoryFactory,
	publisher interfaces.SubscriptionEventPublisher,
	o11y observability.Observability,
) *ProcessSubscriptionCanceled {
	return &ProcessSubscriptionCanceled{uow: u, factory: factory, publisher: publisher, o11y: o11y}
}

func (uc *ProcessSubscriptionCanceled) Execute(ctx context.Context, in input.ProcessSubscriptionCanceledInput) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "billing.usecase.process_subscription_canceled")
	defer span.End()

	eventKey := fmt.Sprintf("subscription_canceled:%s", in.KiwifySubID)

	_, execErr := uc.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (entities.Subscription, error) {
		processedRepo := uc.factory.ProcessedEventRepository(tx)
		subRepo := uc.factory.SubscriptionRepository(tx)

		if markErr := processedRepo.MarkApplied(ctx, eventKey, "subscription_canceled", in.KiwifySubID, in.OccurredAt); markErr != nil {
			if errors.Is(markErr, interfaces.ErrEventAlreadyProcessed) {
				return entities.Subscription{}, ErrEventAlreadyProcessed
			}
			return entities.Subscription{}, fmt.Errorf("billing.usecase.process_subscription_canceled: mark applied: %w", markErr)
		}

		existing, findErr := uc.resolveSubscription(ctx, subRepo, in)
		if findErr != nil {
			return entities.Subscription{}, fmt.Errorf("billing.usecase.process_subscription_canceled: find subscription: %w", findErr)
		}

		transitionSvc := services.NewTransitionService()
		if transitionSvc.IsRegression(existing.Status(), services.TriggerSubscriptionCanceled, in.OccurredAt, existing.LastEventAt()) {
			if supersededErr := processedRepo.MarkSuperseded(ctx, eventKey); supersededErr != nil {
				return entities.Subscription{}, fmt.Errorf("billing.usecase.process_subscription_canceled: mark superseded: %w", supersededErr)
			}
			return existing, ErrEventSuperseded
		}

		if applyErr := subRepo.ApplyTransition(ctx, existing.ID(), valueobjects.StatusCanceledPending, time.Time{}, in.OccurredAt); applyErr != nil {
			return entities.Subscription{}, fmt.Errorf("billing.usecase.process_subscription_canceled: apply transition: %w", applyErr)
		}

		updatedSub := entities.Hydrate(
			existing.ID(),
			existing.FunnelToken(),
			existing.Plan(),
			valueobjects.StatusCanceledPending,
			existing.PeriodStart(),
			existing.PeriodEnd(),
			time.Time{},
			in.OccurredAt,
		)

		if pubErr := uc.publisher.PublishCanceled(ctx, tx, updatedSub, updatedSub.ID()); pubErr != nil {
			return entities.Subscription{}, fmt.Errorf("billing.usecase.process_subscription_canceled: publish canceled: %w", pubErr)
		}

		return updatedSub, nil
	})

	if execErr != nil {
		span.RecordError(execErr)
		if errors.Is(execErr, ErrEventAlreadyProcessed) || errors.Is(execErr, ErrEventSuperseded) {
			return execErr
		}
		uc.o11y.Logger().Error(ctx, "billing.usecase.process_subscription_canceled.failed",
			observability.String("kiwify_sub_id", in.KiwifySubID),
			observability.String("order_id", in.OrderID),
			observability.Error(execErr),
		)
		return execErr
	}

	return nil
}

func (uc *ProcessSubscriptionCanceled) resolveSubscription(ctx context.Context, subRepo interfaces.SubscriptionRepository, in input.ProcessSubscriptionCanceledInput) (entities.Subscription, error) {
	if in.KiwifySubID != "" {
		if existing, err := subRepo.FindByKiwifySubID(ctx, in.KiwifySubID); err == nil {
			return existing, nil
		}
	}
	return subRepo.FindByOrderID(ctx, in.OrderID)
}
