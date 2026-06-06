package usecases

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
)

type ReconcileSubscriptions struct {
	db           database.DBTX
	factory      interfaces.RepositoryFactory
	kiwifyClient interfaces.KiwifyClient
	saleApproved *ProcessSaleApproved
	refund       *ProcessRefundOrChargeback
	o11y         observability.Observability
}

func NewReconcileSubscriptions(
	db database.DBTX,
	factory interfaces.RepositoryFactory,
	kiwifyClient interfaces.KiwifyClient,
	saleApproved *ProcessSaleApproved,
	refund *ProcessRefundOrChargeback,
	o11y observability.Observability,
) *ReconcileSubscriptions {
	return &ReconcileSubscriptions{
		db:           db,
		factory:      factory,
		kiwifyClient: kiwifyClient,
		saleApproved: saleApproved,
		refund:       refund,
		o11y:         o11y,
	}
}

func (uc *ReconcileSubscriptions) Execute(ctx context.Context, in input.ReconcileSubscriptionsInput) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "billing.usecase.reconcile_subscriptions")
	defer span.End()

	for page := 1; ; page++ {
		salesPage, listErr := uc.kiwifyClient.ListSalesUpdatedSince(ctx, in.WindowStart, in.WindowEnd, page)
		if listErr != nil {
			return fmt.Errorf("billing.usecase.reconcile_subscriptions: list sales page %d: %w", page, listErr)
		}

		for _, sale := range salesPage.Sales {
			if err := uc.reconcileSale(ctx, sale); err != nil {
				uc.o11y.Logger().Error(ctx, "billing.usecase.reconcile_subscriptions.sale_failed",
					observability.String("sale_id", sale.ID),
					observability.Error(err),
				)
			}
		}

		if !salesPage.HasMore {
			break
		}
	}

	checkpointRepo := uc.factory.ReconciliationCheckpointRepository(uc.db)
	if setErr := checkpointRepo.Set(ctx, "kiwify_sales", in.WindowEnd); setErr != nil {
		return fmt.Errorf("billing.usecase.reconcile_subscriptions: set checkpoint: %w", setErr)
	}

	return nil
}

func (uc *ReconcileSubscriptions) reconcileSale(ctx context.Context, sale interfaces.KiwifySale) error {
	if sale.Status == "refunded" || sale.Status == "chargedback" {
		trigger := "compra_reembolsada"
		if sale.Status == "chargedback" {
			trigger = "chargeback"
		}
		return uc.refund.Execute(ctx, input.ProcessRefundOrChargebackInput{
			SaleID:     sale.ID,
			OrderID:    sale.OrderID,
			Trigger:    trigger,
			OccurredAt: sale.UpdatedAt,
		})
	}

	if sale.Status == "paid" || sale.Status == "approved" {
		return uc.saleApproved.Execute(ctx, input.ProcessSaleApprovedInput{
			SaleID:          sale.ID,
			KiwifyProductID: sale.KiwifyProductID,
			OrderID:         sale.OrderID,
			FunnelToken:     sale.FunnelToken,
			OccurredAt:      sale.OccurredAt,
		})
	}

	return nil
}
