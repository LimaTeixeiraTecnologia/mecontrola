package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/observability"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

const reconciliationDefaultBatchSize = 200

// ReconcileSubscriptionsUseCase percorre subscriptions ativas em batches,
// detecta divergências com o provedor e publica evento sintético no outbox (RF-39, RF-41).
type ReconcileSubscriptionsUseCase struct {
	subRepo     interfaces.SubscriptionRepository
	webhookRepo interfaces.WebhookEventRepository
	provider    interfaces.BillingProvider
	publisher   outbox.Publisher
	txRunner    interfaces.TxRunner[output.ReconciliationReport]
	idGenerator interfaces.IDGenerator
	o11y        observability.Observability
	metrics     *observability.UsecaseMetrics
	batchSize   int
}

func NewReconcileSubscriptionsUseCase(
	subRepo interfaces.SubscriptionRepository,
	webhookRepo interfaces.WebhookEventRepository,
	provider interfaces.BillingProvider,
	publisher outbox.Publisher,
	txRunner interfaces.TxRunner[output.ReconciliationReport],
	idGenerator interfaces.IDGenerator,
	o11y observability.Observability,
	metrics *observability.UsecaseMetrics,
	batchSize ...int,
) *ReconcileSubscriptionsUseCase {
	effectiveBatchSize := reconciliationDefaultBatchSize
	if len(batchSize) > 0 && batchSize[0] > 0 {
		effectiveBatchSize = batchSize[0]
	}
	return &ReconcileSubscriptionsUseCase{
		subRepo:     subRepo,
		webhookRepo: webhookRepo,
		provider:    provider,
		publisher:   publisher,
		txRunner:    txRunner,
		idGenerator: idGenerator,
		o11y:        o11y,
		metrics:     metrics,
		batchSize:   effectiveBatchSize,
	}
}

func (u *ReconcileSubscriptionsUseCase) Execute(ctx context.Context) (output.ReconciliationReport, error) {
	return observability.Observe(ctx, u.o11y, u.metrics, "billing", "reconcile_subscriptions", func(ctx context.Context) (output.ReconciliationReport, error) {
		return u.execute(ctx)
	})
}

func (u *ReconcileSubscriptionsUseCase) execute(ctx context.Context) (output.ReconciliationReport, error) {
	statuses := []valueobjects.SubscriptionStatus{
		valueobjects.SubscriptionStatusActive,
		valueobjects.SubscriptionStatusPastDue,
	}

	var (
		cursorAt = time.Time{}
		cursorID = entities.SubscriptionID{}
		report   output.ReconciliationReport
	)

	for {
		batch, err := u.subRepo.ListByStatusInBatch(ctx, statuses, cursorAt, cursorID, u.batchSize)
		if err != nil {
			return report, fmt.Errorf("reconciliação: listar batch: %w", err)
		}
		if len(batch) == 0 {
			break
		}

		for _, sub := range batch {
			report.Inspected++
			diverged, err := u.reconcileOne(ctx, sub)
			if err != nil {
				return report, fmt.Errorf("reconciliação: processar subscription %s: %w", sub.ID().String(), err)
			}
			if diverged {
				report.Diverged++
				report.Synced++
			}
		}

		last := batch[len(batch)-1]
		cursorAt = last.CreatedAt()
		cursorID = last.ID()

		if len(batch) < u.batchSize {
			break
		}
	}
	return report, nil
}

func (u *ReconcileSubscriptionsUseCase) reconcileOne(ctx context.Context, sub *entities.Subscription) (bool, error) {
	remote, err := u.provider.FetchSubscription(ctx, sub.ExternalSubscriptionID().String())
	if err != nil {
		return false, fmt.Errorf("buscar subscription remota: %w", err)
	}

	if remote.Status == sub.InternalStatus() && remote.PeriodEnd.Equal(sub.PeriodEnd()) {
		return false, nil
	}

	_, err = u.txRunner.Do(ctx, func(txCtx context.Context, tx database.DBTX) (output.ReconciliationReport, error) {
		now := time.Now().UTC()
		applicationID, err := valueobjects.NewWebhookEventID(u.idGenerator.NewID())
		if err != nil {
			return output.ReconciliationReport{}, fmt.Errorf("gerar application event id: %w", err)
		}
		payload, err := u.buildSyntheticPayload(sub, remote, applicationID, now)
		if err != nil {
			return output.ReconciliationReport{}, err
		}
		eventID, err := events.NewEventID(applicationID.String())
		if err != nil {
			return output.ReconciliationReport{}, fmt.Errorf("gerar event id: %w", err)
		}
		eventName, err := events.NewEventName("billing.reconciliation.divergence_detected")
		if err != nil {
			return output.ReconciliationReport{}, fmt.Errorf("criar event name: %w", err)
		}
		evt, err := outbox.NewEvent(outbox.NewEventParams{
			ID:            eventID,
			EventType:     eventName,
			AggregateType: "subscription",
			AggregateID:   sub.ID().String(),
			Payload:       payload,
			OccurredAt:    now,
		})
		if err != nil {
			return output.ReconciliationReport{}, fmt.Errorf("criar outbox event: %w", err)
		}
		if err := u.publisher.Publish(txCtx, tx, evt); err != nil {
			return output.ReconciliationReport{}, fmt.Errorf("publicar evento sintético: %w", err)
		}
		return output.ReconciliationReport{}, nil
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

func (u *ReconcileSubscriptionsUseCase) buildSyntheticPayload(
	sub *entities.Subscription,
	remote services.CanonicalSubscription,
	applicationID valueobjects.WebhookEventID,
	occurredAt time.Time,
) (json.RawMessage, error) {
	return json.Marshal(reconciliationEventPayload{
		UserID:                 sub.UserID().String(),
		ExternalEventID:        "reconciliation:" + applicationID.String(),
		ExternalSubscriptionID: sub.ExternalSubscriptionID().String(),
		EventType:              eventTypeForRemoteStatus(remote.Status).String(),
		PlanCode:               remote.PlanCode.String(),
		OccurredAt:             occurredAt,
		PeriodStart:            remote.PeriodStart,
		PeriodEnd:              remote.PeriodEnd,
		WhatsAppNumber:         remote.Customer.WhatsApp.String(),
		ApplicationEventID:     applicationID.String(),
		LastWebhookEventID:     sub.LastWebhookEventID().String(),
	})
}

func eventTypeForRemoteStatus(status valueobjects.SubscriptionStatus) valueobjects.CanonicalEventType {
	switch status {
	case valueobjects.SubscriptionStatusTrialing, valueobjects.SubscriptionStatusActive:
		return valueobjects.CanonicalEventRenewed
	case valueobjects.SubscriptionStatusPastDue:
		return valueobjects.CanonicalEventLate
	case valueobjects.SubscriptionStatusCanceledPending:
		return valueobjects.CanonicalEventCanceled
	case valueobjects.SubscriptionStatusExpired:
		return valueobjects.CanonicalEventExpired
	case valueobjects.SubscriptionStatusRefunded:
		return valueobjects.CanonicalEventRefunded
	default:
		return valueobjects.CanonicalEventUnknown
	}
}
