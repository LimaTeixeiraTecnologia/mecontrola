package usecases

import (
	"context"
	"errors"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

type ProcessSaleApproved struct {
	uow       uow.UnitOfWork[entities.Subscription]
	factory   interfaces.RepositoryFactory
	publisher interfaces.SubscriptionEventPublisher
	o11y      observability.Observability
}

func NewProcessSaleApproved(
	u uow.UnitOfWork[entities.Subscription],
	factory interfaces.RepositoryFactory,
	publisher interfaces.SubscriptionEventPublisher,
	o11y observability.Observability,
) *ProcessSaleApproved {
	return &ProcessSaleApproved{uow: u, factory: factory, publisher: publisher, o11y: o11y}
}

func (uc *ProcessSaleApproved) Execute(ctx context.Context, in input.ProcessSaleApprovedInput) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "billing.usecase.process_sale_approved")
	defer span.End()

	if in.FunnelToken == "" {
		return uc.executeWithoutToken(ctx, in)
	}

	funnelToken, err := valueobjects.NewFunnelToken(in.FunnelToken)
	if err != nil {
		return ErrFunnelTokenMissing
	}

	eventKey := fmt.Sprintf("order_approved:%s", in.SaleID)

	_, execErr := uc.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (entities.Subscription, error) {
		processedRepo := uc.factory.ProcessedEventRepository(tx)
		planRepo := uc.factory.PlanRepository(tx)
		subRepo := uc.factory.SubscriptionRepository(tx)

		if markErr := processedRepo.MarkApplied(ctx, eventKey, "order_approved", in.SaleID, in.OccurredAt); markErr != nil {
			if errors.Is(markErr, interfaces.ErrEventAlreadyProcessed) {
				return entities.Subscription{}, ErrEventAlreadyProcessed
			}
			return entities.Subscription{}, fmt.Errorf("billing.usecase.process_sale_approved: mark applied: %w", markErr)
		}

		plan, planErr := planRepo.FindByKiwifyProductID(ctx, in.KiwifyProductID)
		if planErr != nil {
			return entities.Subscription{}, ErrPlanNotFound
		}

		sub := entities.NewSubscription(plan, funnelToken)
		if activateErr := sub.Activate(in.OccurredAt); activateErr != nil {
			return entities.Subscription{}, fmt.Errorf("billing.usecase.process_sale_approved: activate: %w", activateErr)
		}

		periodStart := in.OccurredAt
		if upsertErr := subRepo.UpsertByOrder(ctx, interfaces.UpsertByOrderParams{
			OrderID:            in.OrderID,
			KiwifySubID:        in.KiwifySubID,
			ExternalSaleID:     in.SaleID,
			CustomerMobileE164: in.CustomerMobileE164,
			CustomerEmail:      in.CustomerEmail,
			Subscription:       sub,
			PeriodStart:        periodStart,
		}); upsertErr != nil {
			return entities.Subscription{}, fmt.Errorf("billing.usecase.process_sale_approved: upsert: %w", upsertErr)
		}

		persisted, findErr := subRepo.FindByOrderID(ctx, in.OrderID)
		if findErr != nil {
			return entities.Subscription{}, fmt.Errorf("billing.usecase.process_sale_approved: find after upsert: %w", findErr)
		}

		if pubErr := uc.publisher.PublishActivated(ctx, tx, persisted, persisted.ID(), funnelToken.String(), in.CustomerMobileE164, in.CustomerEmail, in.SaleID); pubErr != nil {
			return entities.Subscription{}, fmt.Errorf("billing.usecase.process_sale_approved: publish activated: %w", pubErr)
		}

		return persisted, nil
	})

	if execErr != nil {
		span.RecordError(execErr)
		if errors.Is(execErr, ErrEventAlreadyProcessed) {
			return ErrEventAlreadyProcessed
		}
		if errors.Is(execErr, ErrFunnelTokenMissing) || errors.Is(execErr, ErrPlanNotFound) {
			return execErr
		}
		uc.o11y.Logger().Error(ctx, "billing.usecase.process_sale_approved.failed",
			observability.String("sale_id", in.SaleID),
			observability.Error(execErr),
		)
		return execErr
	}

	return nil
}

func (uc *ProcessSaleApproved) executeWithoutToken(ctx context.Context, in input.ProcessSaleApprovedInput) error {
	eventKey := fmt.Sprintf("order_approved:%s", in.SaleID)

	_, execErr := uc.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (entities.Subscription, error) {
		processedRepo := uc.factory.ProcessedEventRepository(tx)
		planRepo := uc.factory.PlanRepository(tx)
		subRepo := uc.factory.SubscriptionRepository(tx)

		if markErr := processedRepo.MarkApplied(ctx, eventKey, "order_approved", in.SaleID, in.OccurredAt); markErr != nil {
			if errors.Is(markErr, interfaces.ErrEventAlreadyProcessed) {
				return entities.Subscription{}, ErrEventAlreadyProcessed
			}
			return entities.Subscription{}, fmt.Errorf("billing.usecase.process_sale_approved.without_token: mark applied: %w", markErr)
		}

		plan, planErr := planRepo.FindByKiwifyProductID(ctx, in.KiwifyProductID)
		if planErr != nil {
			return entities.Subscription{}, ErrPlanNotFound
		}

		sub := entities.NewSubscription(plan, valueobjects.FunnelToken{})
		if activateErr := sub.Activate(in.OccurredAt); activateErr != nil {
			return entities.Subscription{}, fmt.Errorf("billing.usecase.process_sale_approved.without_token: activate: %w", activateErr)
		}

		periodStart := in.OccurredAt
		if upsertErr := subRepo.UpsertByOrder(ctx, interfaces.UpsertByOrderParams{
			OrderID:            in.OrderID,
			KiwifySubID:        in.KiwifySubID,
			ExternalSaleID:     in.SaleID,
			CustomerMobileE164: in.CustomerMobileE164,
			CustomerEmail:      in.CustomerEmail,
			Subscription:       sub,
			PeriodStart:        periodStart,
		}); upsertErr != nil {
			return entities.Subscription{}, fmt.Errorf("billing.usecase.process_sale_approved.without_token: upsert: %w", upsertErr)
		}

		persisted, findErr := subRepo.FindByOrderID(ctx, in.OrderID)
		if findErr != nil {
			return entities.Subscription{}, fmt.Errorf("billing.usecase.process_sale_approved.without_token: find after upsert: %w", findErr)
		}

		if pubErr := uc.publisher.PublishActivatedWithoutToken(ctx, tx, persisted, persisted.ID(), in.CustomerMobileE164, in.CustomerEmail, in.SaleID); pubErr != nil {
			return entities.Subscription{}, fmt.Errorf("billing.usecase.process_sale_approved.without_token: publish: %w", pubErr)
		}

		return persisted, nil
	})

	if execErr != nil {
		if errors.Is(execErr, ErrEventAlreadyProcessed) {
			return ErrEventAlreadyProcessed
		}
		if errors.Is(execErr, ErrPlanNotFound) {
			return execErr
		}
		uc.o11y.Logger().Error(ctx, "billing.usecase.process_sale_approved.without_token.failed",
			observability.String("sale_id", in.SaleID),
			observability.Error(execErr),
		)
		return execErr
	}

	return nil
}
