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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

type ProcessRefundOrChargeback struct {
	uow       uow.UnitOfWork[entities.Subscription]
	factory   interfaces.RepositoryFactory
	publisher interfaces.SubscriptionEventPublisher
	o11y      observability.Observability
}

func NewProcessRefundOrChargeback(
	u uow.UnitOfWork[entities.Subscription],
	factory interfaces.RepositoryFactory,
	publisher interfaces.SubscriptionEventPublisher,
	o11y observability.Observability,
) *ProcessRefundOrChargeback {
	return &ProcessRefundOrChargeback{uow: u, factory: factory, publisher: publisher, o11y: o11y}
}

func (uc *ProcessRefundOrChargeback) Execute(ctx context.Context, in input.ProcessRefundOrChargebackInput) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "billing.usecase.process_refund_or_chargeback")
	defer span.End()

	trigger := in.Trigger
	if trigger == "" {
		trigger = "order_refunded"
	}
	eventKey := fmt.Sprintf("%s:%s", trigger, in.SaleID)

	_, execErr := uc.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (entities.Subscription, error) {
		processedRepo := uc.factory.ProcessedEventRepository(tx)
		subRepo := uc.factory.SubscriptionRepository(tx)

		if markErr := processedRepo.MarkApplied(ctx, eventKey, in.Trigger, in.SaleID, in.OccurredAt); markErr != nil {
			if errors.Is(markErr, interfaces.ErrEventAlreadyProcessed) {
				return entities.Subscription{}, ErrEventAlreadyProcessed
			}
			return entities.Subscription{}, fmt.Errorf("billing.usecase.process_refund_or_chargeback: mark applied: %w", markErr)
		}

		existing, findErr := subRepo.FindByOrderID(ctx, in.OrderID)
		if findErr != nil {
			return entities.Subscription{}, fmt.Errorf("billing.usecase.process_refund_or_chargeback: find subscription: %w", findErr)
		}

		if applyErr := subRepo.ApplyTransition(ctx, existing.ID(), valueobjects.StatusRefunded, time.Time{}, in.OccurredAt); applyErr != nil {
			return entities.Subscription{}, fmt.Errorf("billing.usecase.process_refund_or_chargeback: apply transition: %w", applyErr)
		}

		updatedSub := entities.HydrateWithUser(
			existing.ID(),
			existing.UserID(),
			existing.FunnelToken(),
			existing.Plan(),
			valueobjects.StatusRefunded,
			existing.PeriodStart(),
			existing.PeriodEnd(),
			time.Time{},
			in.OccurredAt,
		)

		if pubErr := uc.publisher.PublishRefunded(ctx, tx, updatedSub, updatedSub.ID()); pubErr != nil {
			return entities.Subscription{}, fmt.Errorf("billing.usecase.process_refund_or_chargeback: publish refunded: %w", pubErr)
		}

		return updatedSub, nil
	})

	if execErr != nil {
		span.RecordError(execErr)
		if errors.Is(execErr, ErrEventAlreadyProcessed) {
			return execErr
		}
		uc.o11y.Logger().Error(ctx, "billing.usecase.process_refund_or_chargeback.failed",
			observability.String("sale_id", in.SaleID),
			observability.Error(execErr),
		)
		return execErr
	}

	return nil
}
