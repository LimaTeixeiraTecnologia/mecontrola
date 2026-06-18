package usecases

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

var ErrRenewedWithoutBaseSubscription = errors.New("billing: renewal received for unknown kiwify_subscription_id")

type ProcessSubscriptionRenewed struct {
	uow       uow.UnitOfWork
	factory   interfaces.RepositoryFactory
	publisher interfaces.SubscriptionEventPublisher
	o11y      observability.Observability
}

func NewProcessSubscriptionRenewed(
	u uow.UnitOfWork,
	factory interfaces.RepositoryFactory,
	publisher interfaces.SubscriptionEventPublisher,
	o11y observability.Observability,
) *ProcessSubscriptionRenewed {
	return &ProcessSubscriptionRenewed{uow: u, factory: factory, publisher: publisher, o11y: o11y}
}

func (uc *ProcessSubscriptionRenewed) Execute(ctx context.Context, in input.ProcessSubscriptionRenewedInput) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "billing.usecase.process_subscription_renewed")
	defer span.End()

	kiwifySubID, err := valueobjects.NewKiwifySubscriptionID(in.KiwifySubID)
	if err != nil {
		return ErrKiwifySubscriptionIDInvalid
	}

	eventKey := fmt.Sprintf("subscription_renewed:%s:%s", kiwifySubID.String(), in.OccurredAt.UTC().Format("2006-01-02T15:04:05Z07:00"))

	_, execErr := uow.Do(ctx, uc.uow, func(ctx context.Context, tx database.DBTX) (entities.Subscription, error) {
		return uc.applyRenewal(ctx, tx, in, kiwifySubID, eventKey)
	})

	if execErr != nil {
		span.RecordError(execErr)
		if errors.Is(execErr, ErrEventAlreadyProcessed) || errors.Is(execErr, ErrEventSuperseded) || errors.Is(execErr, ErrRenewedWithoutBaseSubscription) {
			return execErr
		}
		uc.o11y.Logger().Error(ctx, "billing.usecase.process_subscription_renewed.failed",
			observability.String("kiwify_sub_id", in.KiwifySubID),
			observability.String("order_id", in.OrderID),
			observability.Error(execErr),
		)
		return execErr
	}

	return nil
}

func (uc *ProcessSubscriptionRenewed) applyRenewal(
	ctx context.Context,
	tx database.DBTX,
	in input.ProcessSubscriptionRenewedInput,
	kiwifySubID valueobjects.KiwifySubscriptionID,
	eventKey string,
) (entities.Subscription, error) {
	processedRepo := uc.factory.ProcessedEventRepository(tx)
	subRepo := uc.factory.SubscriptionRepository(tx)

	if markErr := processedRepo.MarkApplied(ctx, eventKey, "subscription_renewed", kiwifySubID.String(), in.OccurredAt); markErr != nil {
		if errors.Is(markErr, interfaces.ErrEventAlreadyProcessed) {
			return entities.Subscription{}, ErrEventAlreadyProcessed
		}
		return entities.Subscription{}, fmt.Errorf("billing.usecase.process_subscription_renewed: mark applied: %w", markErr)
	}

	existing, findErr := subRepo.FindByKiwifySubID(ctx, kiwifySubID.String())
	if findErr != nil {
		uc.o11y.Logger().Warn(ctx, "billing.usecase.process_subscription_renewed.base_subscription_missing",
			observability.String("kiwify_sub_id", kiwifySubID.String()),
			observability.String("order_id", in.OrderID),
		)
		return entities.Subscription{}, fmt.Errorf("billing.usecase.process_subscription_renewed: %w", ErrRenewedWithoutBaseSubscription)
	}

	return uc.extendExisting(ctx, tx, subRepo, processedRepo, existing, in, eventKey)
}

func (uc *ProcessSubscriptionRenewed) extendExisting(ctx context.Context, tx database.DBTX, subRepo interfaces.SubscriptionRepository, processedRepo interfaces.ProcessedEventRepository, existing entities.Subscription, in input.ProcessSubscriptionRenewedInput, eventKey string) (entities.Subscription, error) {
	transitionSvc := services.NewTransitionService()
	if transitionSvc.DecideRenewal(existing.Status(), in.OccurredAt, existing.LastEventAt()) == services.DecisionSkipAsRegression {
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

	renewed := entities.HydrateWithUser(
		existing.ID(),
		existing.UserID(),
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
