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

type ProcessSubscriptionRenewed struct {
	uow       uow.UnitOfWork[entities.Subscription]
	factory   interfaces.RepositoryFactory
	publisher interfaces.SubscriptionEventPublisher
	o11y      observability.Observability
}

func NewProcessSubscriptionRenewed(
	u uow.UnitOfWork[entities.Subscription],
	factory interfaces.RepositoryFactory,
	publisher interfaces.SubscriptionEventPublisher,
	o11y observability.Observability,
) *ProcessSubscriptionRenewed {
	return &ProcessSubscriptionRenewed{uow: u, factory: factory, publisher: publisher, o11y: o11y}
}

func (uc *ProcessSubscriptionRenewed) Execute(ctx context.Context, in input.ProcessSubscriptionRenewedInput) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "billing.usecase.process_subscription_renewed")
	defer span.End()

	eventKey := fmt.Sprintf("subscription_renewed:%s:%s", in.KiwifySubID, in.OccurredAt.UTC().Format("2006-01-02T15:04:05Z07:00"))

	_, execErr := uc.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (entities.Subscription, error) {
		return uc.applyRenewal(ctx, tx, in, eventKey)
	})

	if execErr != nil {
		span.RecordError(execErr)
		if errors.Is(execErr, ErrEventAlreadyProcessed) || errors.Is(execErr, ErrEventSuperseded) {
			return execErr
		}
		uc.o11y.Logger().Error(ctx, "billing.usecase.process_subscription_renewed.failed",
			observability.String("order_id", in.OrderID),
			observability.Error(execErr),
		)
		return execErr
	}

	return nil
}

func (uc *ProcessSubscriptionRenewed) applyRenewal(ctx context.Context, tx database.DBTX, in input.ProcessSubscriptionRenewedInput, eventKey string) (entities.Subscription, error) {
	processedRepo := uc.factory.ProcessedEventRepository(tx)
	planRepo := uc.factory.PlanRepository(tx)
	subRepo := uc.factory.SubscriptionRepository(tx)

	if markErr := processedRepo.MarkApplied(ctx, eventKey, "subscription_renewed", in.KiwifySubID, in.OccurredAt); markErr != nil {
		if errors.Is(markErr, interfaces.ErrEventAlreadyProcessed) {
			return entities.Subscription{}, ErrEventAlreadyProcessed
		}
		return entities.Subscription{}, fmt.Errorf("billing.usecase.process_subscription_renewed: mark applied: %w", markErr)
	}

	existing, findErr := subRepo.FindByOrderID(ctx, in.OrderID)
	if findErr != nil {
		return uc.createPlaceholder(ctx, tx, subRepo, planRepo, in)
	}

	return uc.extendExisting(ctx, tx, subRepo, processedRepo, existing, in, eventKey)
}

func (uc *ProcessSubscriptionRenewed) createPlaceholder(ctx context.Context, tx database.DBTX, subRepo interfaces.SubscriptionRepository, planRepo interfaces.PlanRepository, in input.ProcessSubscriptionRenewedInput) (entities.Subscription, error) {
	plan, planErr := planRepo.FindByKiwifyProductID(ctx, in.KiwifyProductID)
	if planErr != nil {
		return entities.Subscription{}, ErrPlanNotFound
	}

	sub := entities.NewSubscription(plan, valueobjects.FunnelToken{})
	if activateErr := sub.Activate(in.OccurredAt); activateErr != nil {
		return entities.Subscription{}, fmt.Errorf("billing.usecase.process_subscription_renewed: activate placeholder: %w", activateErr)
	}

	if upsertErr := subRepo.UpsertByOrder(ctx, in.OrderID, sub, in.OccurredAt); upsertErr != nil {
		return entities.Subscription{}, fmt.Errorf("billing.usecase.process_subscription_renewed: upsert placeholder: %w", upsertErr)
	}

	persisted, findNewErr := subRepo.FindByOrderID(ctx, in.OrderID)
	if findNewErr != nil {
		return entities.Subscription{}, fmt.Errorf("billing.usecase.process_subscription_renewed: find placeholder: %w", findNewErr)
	}

	if pubErr := uc.publisher.PublishRenewed(ctx, tx, persisted, persisted.ID(), in.OccurredAt); pubErr != nil {
		return entities.Subscription{}, fmt.Errorf("billing.usecase.process_subscription_renewed: publish renewed placeholder: %w", pubErr)
	}

	return persisted, nil
}

func (uc *ProcessSubscriptionRenewed) extendExisting(ctx context.Context, tx database.DBTX, subRepo interfaces.SubscriptionRepository, processedRepo interfaces.ProcessedEventRepository, existing entities.Subscription, in input.ProcessSubscriptionRenewedInput, eventKey string) (entities.Subscription, error) {
	transitionSvc := services.NewTransitionService()
	if transitionSvc.IsRegression(existing.Status(), services.TriggerSubscriptionRenewed, in.OccurredAt, existing.LastEventAt()) {
		if supersededErr := processedRepo.MarkSuperseded(ctx, eventKey); supersededErr != nil {
			return entities.Subscription{}, fmt.Errorf("billing.usecase.process_subscription_renewed: mark superseded: %w", supersededErr)
		}
		return existing, ErrEventSuperseded
	}

	previousPeriodEnd := existing.PeriodEnd()
	newPeriodEnd := previousPeriodEnd.Add(existing.Plan().Duration())

	if extendErr := subRepo.ExtendPeriod(ctx, existing.ID(), newPeriodEnd, in.OccurredAt); extendErr != nil {
		return entities.Subscription{}, fmt.Errorf("billing.usecase.process_subscription_renewed: extend period: %w", extendErr)
	}

	renewed := entities.Hydrate(
		existing.ID(),
		existing.FunnelToken(),
		existing.Plan(),
		valueobjects.StatusActive,
		existing.PeriodStart(),
		newPeriodEnd,
		time.Time{},
		in.OccurredAt,
	)

	if pubErr := uc.publisher.PublishRenewed(ctx, tx, renewed, renewed.ID(), previousPeriodEnd); pubErr != nil {
		return entities.Subscription{}, fmt.Errorf("billing.usecase.process_subscription_renewed: publish renewed: %w", pubErr)
	}

	return renewed, nil
}
