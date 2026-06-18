package usecases

import (
	"context"
	"errors"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

type ProcessSubscriptionLate struct {
	uow       uow.UnitOfWork
	factory   interfaces.RepositoryFactory
	publisher interfaces.SubscriptionEventPublisher
	o11y      observability.Observability
}

func NewProcessSubscriptionLate(
	u uow.UnitOfWork,
	factory interfaces.RepositoryFactory,
	publisher interfaces.SubscriptionEventPublisher,
	o11y observability.Observability,
) *ProcessSubscriptionLate {
	return &ProcessSubscriptionLate{uow: u, factory: factory, publisher: publisher, o11y: o11y}
}

func (uc *ProcessSubscriptionLate) Execute(ctx context.Context, in input.ProcessSubscriptionLateInput) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "billing.usecase.process_subscription_late")
	defer span.End()

	eventKey := fmt.Sprintf("subscription_late:%s:%s", in.KiwifySubID, in.OccurredAt.UTC().Format("2006-01-02T15:04:05Z07:00"))

	_, execErr := uow.Do(ctx, uc.uow, func(ctx context.Context, tx database.DBTX) (entities.Subscription, error) {
		processedRepo := uc.factory.ProcessedEventRepository(tx)
		subRepo := uc.factory.SubscriptionRepository(tx)

		if markErr := processedRepo.MarkApplied(ctx, eventKey, "subscription_late", in.KiwifySubID, in.OccurredAt); markErr != nil {
			if errors.Is(markErr, interfaces.ErrEventAlreadyProcessed) {
				return entities.Subscription{}, ErrEventAlreadyProcessed
			}
			return entities.Subscription{}, fmt.Errorf("billing.usecase.process_subscription_late: mark applied: %w", markErr)
		}

		existing, findErr := uc.resolveSubscription(ctx, subRepo, in)
		if findErr != nil {
			return entities.Subscription{}, fmt.Errorf("billing.usecase.process_subscription_late: find subscription: %w", findErr)
		}

		transitionSvc := services.NewTransitionService()
		if transitionSvc.DecidePastDue(existing.Status(), in.OccurredAt, existing.LastEventAt()) == services.DecisionSkipAsRegression {
			if supersededErr := processedRepo.MarkSuperseded(ctx, eventKey); supersededErr != nil {
				return entities.Subscription{}, fmt.Errorf("billing.usecase.process_subscription_late: mark superseded: %w", supersededErr)
			}
			return existing, ErrEventSuperseded
		}

		graceEnd := in.OccurredAt.Add(valueobjects.DefaultGraceWindow.Duration())
		if applyErr := subRepo.ApplyTransition(ctx, existing.ID(), valueobjects.StatusPastDue, graceEnd, in.OccurredAt); applyErr != nil {
			return entities.Subscription{}, fmt.Errorf("billing.usecase.process_subscription_late: apply transition: %w", applyErr)
		}

		updatedSub := entities.HydrateWithUser(
			existing.ID(),
			existing.UserID(),
			existing.FunnelToken(),
			existing.Plan(),
			valueobjects.StatusPastDue,
			existing.PeriodStart(),
			existing.PeriodEnd(),
			graceEnd,
			in.OccurredAt,
		)

		if pubErr := uc.publisher.PublishPastDue(ctx, tx, updatedSub, updatedSub.ID()); pubErr != nil {
			return entities.Subscription{}, fmt.Errorf("billing.usecase.process_subscription_late: publish past_due: %w", pubErr)
		}

		return updatedSub, nil
	})

	if execErr != nil {
		span.RecordError(execErr)
		if errors.Is(execErr, ErrEventAlreadyProcessed) || errors.Is(execErr, ErrEventSuperseded) {
			return execErr
		}
		uc.o11y.Logger().Error(ctx, "billing.usecase.process_subscription_late.failed",
			observability.String("kiwify_sub_id", in.KiwifySubID),
			observability.String("order_id", in.OrderID),
			observability.Error(execErr),
		)
		return execErr
	}

	return nil
}

func (uc *ProcessSubscriptionLate) resolveSubscription(ctx context.Context, subRepo interfaces.SubscriptionRepository, in input.ProcessSubscriptionLateInput) (entities.Subscription, error) {
	if in.KiwifySubID != "" {
		if existing, err := subRepo.FindByKiwifySubID(ctx, in.KiwifySubID); err == nil {
			return existing, nil
		}
	}
	return subRepo.FindByOrderID(ctx, in.OrderID)
}
