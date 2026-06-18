package usecases

import (
	"context"
	"errors"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
)

const reconcileMaxPages = 1000

var ErrReconcileMaxPagesExceeded = errors.New("billing: reconciliation page guard tripped")

type ReconcileSubscriptions struct {
	db           database.DBTX
	factory      interfaces.RepositoryFactory
	kiwifyClient interfaces.KiwifyClient
	saleApproved *ProcessSaleApproved
	refund       *ProcessRefundOrChargeback
	o11y         observability.Observability
	corrections  observability.Counter
}

func NewReconcileSubscriptions(
	db database.DBTX,
	factory interfaces.RepositoryFactory,
	kiwifyClient interfaces.KiwifyClient,
	saleApproved *ProcessSaleApproved,
	refund *ProcessRefundOrChargeback,
	o11y observability.Observability,
) *ReconcileSubscriptions {
	corrections := o11y.Metrics().Counter(
		"billing_reconciliation_corrections_total",
		"Total de correções aplicadas durante reconciliação",
		"1",
	)
	return &ReconcileSubscriptions{
		db:           db,
		factory:      factory,
		kiwifyClient: kiwifyClient,
		saleApproved: saleApproved,
		refund:       refund,
		o11y:         o11y,
		corrections:  corrections,
	}
}

func (uc *ReconcileSubscriptions) Execute(ctx context.Context, in input.ReconcileSubscriptionsInput) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "billing.usecase.reconcile_subscriptions")
	defer span.End()

	var saleErrors []error
	for page := 1; page <= reconcileMaxPages; page++ {
		salesPage, listErr := uc.kiwifyClient.ListSalesUpdatedSince(ctx, in.WindowStart, in.WindowEnd, page)
		if listErr != nil {
			return fmt.Errorf("billing.usecase.reconcile_subscriptions: list sales page %d: %w", page, listErr)
		}

		for _, sale := range salesPage.Sales {
			if err := uc.reconcileSale(ctx, sale); err != nil {
				if errors.Is(err, ErrEventAlreadyProcessed) || errors.Is(err, ErrEventSuperseded) {
					continue
				}
				uc.o11y.Logger().Error(ctx, "billing.usecase.reconcile_subscriptions.sale_failed",
					observability.String("sale_id", sale.ID),
					observability.Error(err),
				)
				saleErrors = append(saleErrors, fmt.Errorf("sale %s: %w", sale.ID, err))
				continue
			}
			if _, ok := resolveReconcileAction(sale); ok {
				uc.corrections.Add(ctx, 1, observability.String("correction_type", sale.Status))
			}
		}

		if !salesPage.HasMore {
			break
		}
		if page == reconcileMaxPages {
			uc.o11y.Logger().Error(ctx, "billing.usecase.reconcile_subscriptions.max_pages_reached",
				observability.Int("max_pages", reconcileMaxPages),
			)
			return fmt.Errorf("billing.usecase.reconcile_subscriptions: %w: stopped at %d pages", ErrReconcileMaxPagesExceeded, reconcileMaxPages)
		}
	}

	if err := errors.Join(saleErrors...); err != nil {
		return fmt.Errorf("billing.usecase.reconcile_subscriptions: reconcile sales: %w", err)
	}

	checkpointRepo := uc.factory.ReconciliationCheckpointRepository(uc.db)
	if setErr := checkpointRepo.Set(ctx, "kiwify_sales", in.WindowEnd); setErr != nil {
		return fmt.Errorf("billing.usecase.reconcile_subscriptions: set checkpoint: %w", setErr)
	}

	return nil
}

type reconcileAction struct {
	trigger string
	refund  bool
}

func resolveReconcileAction(sale interfaces.KiwifySale) (reconcileAction, bool) {
	switch sale.Status {
	case "refunded":
		return reconcileAction{trigger: "order_refunded", refund: true}, true
	case "chargedback":
		return reconcileAction{trigger: "chargeback", refund: true}, true
	case "paid", "approved":
		return reconcileAction{trigger: sale.Status, refund: false}, true
	default:
		return reconcileAction{}, false
	}
}

func (uc *ReconcileSubscriptions) reconcileSale(ctx context.Context, sale interfaces.KiwifySale) error {
	action, ok := resolveReconcileAction(sale)
	if !ok {
		return nil
	}
	if action.refund {
		return uc.refund.Execute(ctx, input.ProcessRefundOrChargebackInput{
			SaleID:     sale.ID,
			OrderID:    sale.OrderID,
			Trigger:    action.trigger,
			OccurredAt: sale.UpdatedAt,
		})
	}
	return uc.saleApproved.Execute(ctx, input.ProcessSaleApprovedInput{
		SaleID:             sale.ID,
		KiwifyProductID:    sale.KiwifyProductID,
		OrderID:            sale.OrderID,
		KiwifySubID:        sale.SubscriptionID,
		FunnelToken:        sale.FunnelToken,
		CustomerEmail:      sale.CustomerEmail,
		CustomerMobileE164: sale.CustomerMobileE164,
		OccurredAt:         sale.OccurredAt,
	})
}
