package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
	identityentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	identityvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/observability"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

// ErrSubscriptionNotFound é o sentinel retornado por SubscriptionRepository quando nenhuma
// subscription ativa é encontrada. Definido em interfaces para ser compartilhado pelo repositório.
var ErrSubscriptionNotFound = interfaces.ErrSubscriptionNotFound

type ProcessBillingEventResult struct {
	Subscription     *entities.Subscription
	PreviousState    valueobjects.SubscriptionStatus
	TransitionReason valueobjects.TransitionReason
	WhatsAppNumber   string
	Applied          bool
}

type reconciliationEventPayload struct {
	UserID                 string    `json:"user_id"`
	ExternalEventID        string    `json:"external_event_id"`
	ExternalSubscriptionID string    `json:"external_subscription_id"`
	EventType              string    `json:"event_type"`
	PlanCode               string    `json:"plan_code"`
	OccurredAt             time.Time `json:"occurred_at"`
	PeriodStart            time.Time `json:"period_start"`
	PeriodEnd              time.Time `json:"period_end"`
	WhatsAppNumber         string    `json:"whatsapp_number"`
	RefundAmountCents      int64     `json:"refund_amount_cents"`
	ApplicationEventID     string    `json:"application_event_id"`
	LastWebhookEventID     string    `json:"last_webhook_event_id"`
}

// ProcessBillingEventUseCase implementa outbox.Handler para event_type "billing.kiwify.received".
// É o único mutador de Subscription (ADR-009). Idempotente por evt.ID() via billing_event_applications.
type ProcessBillingEventUseCase struct {
	webhookRepo  interfaces.WebhookEventRepository
	subRepo      interfaces.SubscriptionRepository
	provider     interfaces.BillingProvider
	userResolver interfaces.UserResolver
	cache        interfaces.EntitlementCache
	bus          *events.Bus
	txRunner     interfaces.TxRunner[ProcessBillingEventResult]
	idGenerator  interfaces.IDGenerator
	logger       *slog.Logger
	o11y         observability.Observability
	metrics      *observability.UsecaseMetrics
}

func NewProcessBillingEventUseCase(
	webhookRepo interfaces.WebhookEventRepository,
	subRepo interfaces.SubscriptionRepository,
	provider interfaces.BillingProvider,
	userResolver interfaces.UserResolver,
	cache interfaces.EntitlementCache,
	bus *events.Bus,
	txRunner interfaces.TxRunner[ProcessBillingEventResult],
	idGenerator interfaces.IDGenerator,
	logger *slog.Logger,
	o11y observability.Observability,
	metrics *observability.UsecaseMetrics,
) *ProcessBillingEventUseCase {
	return &ProcessBillingEventUseCase{
		webhookRepo:  webhookRepo,
		subRepo:      subRepo,
		provider:     provider,
		userResolver: userResolver,
		cache:        cache,
		bus:          bus,
		txRunner:     txRunner,
		idGenerator:  idGenerator,
		logger:       logger,
		o11y:         o11y,
		metrics:      metrics,
	}
}

// Handle implementa a assinatura compatível com outbox.Handler (RF-21, ADR-009).
func (p *ProcessBillingEventUseCase) Handle(ctx context.Context, evt outbox.Event) error {
	_, err := observability.Observe(ctx, p.o11y, p.metrics, "billing", "process_billing_event", func(ctx context.Context) (struct{}, error) {
		webhookID, canonical, userID, lastWebhookID, err := p.resolveEvent(ctx, evt)
		if err != nil {
			return struct{}{}, err
		}
		now := time.Now().UTC()
		result, err := p.txRunner.Do(ctx, func(txCtx context.Context, _ database.DBTX) (ProcessBillingEventResult, error) {
			return p.executeTx(txCtx, webhookID, canonical, userID, lastWebhookID, now)
		})
		if err != nil {
			return struct{}{}, err
		}
		if result.Applied && result.Subscription != nil {
			p.cache.Invalidate(result.Subscription.UserID())
			_ = events.Publish(p.bus, ctx, newStateChangedEvent(result, now))
		}
		return struct{}{}, nil
	})
	return err
}

func (p *ProcessBillingEventUseCase) resolveEvent(
	ctx context.Context,
	evt outbox.Event,
) (valueobjects.WebhookEventID, services.CanonicalEvent, identityentities.UserID, valueobjects.WebhookEventID, error) {
	if synthetic, ok, err := decodeReconciliationPayload(evt.Payload()); ok || err != nil {
		if err != nil {
			return valueobjects.WebhookEventID{}, services.CanonicalEvent{}, identityentities.UserID{}, valueobjects.WebhookEventID{},
				fmt.Errorf("process billing event: payload reconciliação inválido: %w: %w", err, outbox.ErrPermanent)
		}
		return synthetic.webhookID, synthetic.canonical, synthetic.userID, synthetic.lastWebhookID, nil
	}

	rawPayload, err := decodeReceivedPayload(evt.Payload())
	if err != nil {
		return valueobjects.WebhookEventID{}, services.CanonicalEvent{}, identityentities.UserID{}, valueobjects.WebhookEventID{},
			fmt.Errorf("process billing event: %w: %w", err, outbox.ErrPermanent)
	}
	webhookID, err := valueobjects.NewWebhookEventID(rawPayload.WebhookEventID)
	if err != nil {
		return valueobjects.WebhookEventID{}, services.CanonicalEvent{}, identityentities.UserID{}, valueobjects.WebhookEventID{},
			fmt.Errorf("process billing event: webhook event id inválido: %w: %w", err, outbox.ErrPermanent)
	}
	raw, err := p.webhookRepo.FindRawPayload(ctx, webhookID)
	if err != nil {
		return valueobjects.WebhookEventID{}, services.CanonicalEvent{}, identityentities.UserID{}, valueobjects.WebhookEventID{},
			fmt.Errorf("process billing event: buscar payload bruto: %w", err)
	}
	canonical, err := p.provider.ParseEvent(raw)
	if err != nil {
		if errors.Is(err, interfaces.ErrUnknownProviderEventType) {
			return valueobjects.WebhookEventID{}, services.CanonicalEvent{}, identityentities.UserID{}, valueobjects.WebhookEventID{},
				fmt.Errorf("process billing event: parse: %w", err)
		}
		return valueobjects.WebhookEventID{}, services.CanonicalEvent{}, identityentities.UserID{}, valueobjects.WebhookEventID{},
			fmt.Errorf("process billing event: parse: %w: %w", err, outbox.ErrPermanent)
	}
	return webhookID, canonical, identityentities.UserID{}, webhookID, nil
}

func (p *ProcessBillingEventUseCase) executeTx(
	ctx context.Context,
	webhookID valueobjects.WebhookEventID,
	canonical services.CanonicalEvent,
	reconciledUserID identityentities.UserID,
	lastWebhookID valueobjects.WebhookEventID,
	now time.Time,
) (ProcessBillingEventResult, error) {
	userID := reconciledUserID
	if userID.String() == "" {
		user, err := p.userResolver.UpsertByWhatsAppNumber(ctx, canonical.Customer.WhatsApp)
		if err != nil {
			return ProcessBillingEventResult{}, fmt.Errorf("upsert user: %w", err)
		}
		userID = user.ID()
	}

	existing, err := p.subRepo.FindActiveByUserIDForUpdate(ctx, userID)
	if err != nil && !errors.Is(err, ErrSubscriptionNotFound) {
		return ProcessBillingEventResult{}, fmt.Errorf("buscar subscription ativa: %w", err)
	}

	if isStaleEvent(existing, canonical.OccurredAt) {
		return ProcessBillingEventResult{}, nil
	}

	previous := valueobjects.SubscriptionStatusUnknown
	if existing != nil {
		previous = existing.InternalStatus()
	}
	updated, applied, reason, err := p.applyCanonicalToSub(existing, canonical, userID, lastWebhookID, now)
	if err != nil {
		return ProcessBillingEventResult{}, fmt.Errorf("aplicar evento canônico: %w: %w", err, outbox.ErrPermanent)
	}
	if !applied {
		return ProcessBillingEventResult{}, nil
	}

	recorded, err := p.webhookRepo.RecordApplication(ctx, webhookID, updated.ID(), now)
	if err != nil {
		return ProcessBillingEventResult{}, fmt.Errorf("registrar aplicação: %w", err)
	}
	if !recorded {
		return ProcessBillingEventResult{}, nil
	}

	if err := p.subRepo.Upsert(ctx, updated); err != nil {
		return ProcessBillingEventResult{}, fmt.Errorf("upsert subscription: %w", err)
	}
	if lastWebhookID.String() == webhookID.String() {
		if err := p.webhookRepo.MarkProcessed(ctx, webhookID, now); err != nil {
			return ProcessBillingEventResult{}, fmt.Errorf("marcar webhook processado: %w", err)
		}
	}
	return ProcessBillingEventResult{
		Subscription:     updated,
		PreviousState:    previous,
		TransitionReason: reason,
		WhatsAppNumber:   canonical.Customer.WhatsApp.String(),
		Applied:          true,
	}, nil
}

type decodedReconciliationEvent struct {
	webhookID     valueobjects.WebhookEventID
	lastWebhookID valueobjects.WebhookEventID
	userID        identityentities.UserID
	canonical     services.CanonicalEvent
}

func decodeReconciliationPayload(raw []byte) (decodedReconciliationEvent, bool, error) {
	var payload reconciliationEventPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return decodedReconciliationEvent{}, false, nil
	}
	if payload.ApplicationEventID == "" {
		return decodedReconciliationEvent{}, false, nil
	}
	userID, err := identityentities.NewUserID(payload.UserID)
	if err != nil {
		return decodedReconciliationEvent{}, true, fmt.Errorf("user_id: %w", err)
	}
	webhookID, err := valueobjects.NewWebhookEventID(payload.ApplicationEventID)
	if err != nil {
		return decodedReconciliationEvent{}, true, fmt.Errorf("application_event_id: %w", err)
	}
	lastWebhookID, err := valueobjects.NewWebhookEventID(payload.LastWebhookEventID)
	if err != nil {
		return decodedReconciliationEvent{}, true, fmt.Errorf("last_webhook_event_id: %w", err)
	}
	plan, err := valueobjects.ParsePlanCode(payload.PlanCode)
	if err != nil {
		return decodedReconciliationEvent{}, true, fmt.Errorf("plan_code: %w", err)
	}
	eventType, err := parseCanonicalEventType(payload.EventType)
	if err != nil {
		return decodedReconciliationEvent{}, true, err
	}
	whatsapp, err := parseOptionalWhatsApp(payload.WhatsAppNumber)
	if err != nil {
		return decodedReconciliationEvent{}, true, err
	}
	return decodedReconciliationEvent{
		webhookID:     webhookID,
		lastWebhookID: lastWebhookID,
		userID:        userID,
		canonical: services.CanonicalEvent{
			Type:                   eventType,
			ExternalEventID:        payload.ExternalEventID,
			ExternalSubscriptionID: payload.ExternalSubscriptionID,
			PlanCode:               plan,
			OccurredAt:             payload.OccurredAt,
			PeriodStart:            payload.PeriodStart,
			PeriodEnd:              payload.PeriodEnd,
			Customer: services.CanonicalCustomer{
				WhatsApp: whatsapp,
			},
			RefundAmountCents: payload.RefundAmountCents,
		},
	}, true, nil
}

func parseOptionalWhatsApp(raw string) (identityvo.WhatsAppNumber, error) {
	if raw == "" {
		return identityvo.WhatsAppNumber{}, nil
	}
	whatsapp, err := identityvo.NewWhatsAppNumber(raw)
	if err != nil {
		return identityvo.WhatsAppNumber{}, fmt.Errorf("whatsapp_number: %w", err)
	}
	return whatsapp, nil
}

func parseCanonicalEventType(raw string) (valueobjects.CanonicalEventType, error) {
	switch raw {
	case valueobjects.CanonicalEventPurchaseApproved.String():
		return valueobjects.CanonicalEventPurchaseApproved, nil
	case valueobjects.CanonicalEventRenewed.String():
		return valueobjects.CanonicalEventRenewed, nil
	case valueobjects.CanonicalEventLate.String():
		return valueobjects.CanonicalEventLate, nil
	case valueobjects.CanonicalEventCanceled.String():
		return valueobjects.CanonicalEventCanceled, nil
	case valueobjects.CanonicalEventRefunded.String():
		return valueobjects.CanonicalEventRefunded, nil
	case valueobjects.CanonicalEventChargeback.String():
		return valueobjects.CanonicalEventChargeback, nil
	case valueobjects.CanonicalEventExpired.String():
		return valueobjects.CanonicalEventExpired, nil
	default:
		return valueobjects.CanonicalEventUnknown, fmt.Errorf("event_type desconhecido: %s", raw)
	}
}

func newStateChangedEvent(result ProcessBillingEventResult, occurredAt time.Time) output.StateChangedEvent {
	var grace *time.Time
	if !result.Subscription.GracePeriodEnd().IsZero() {
		graceAt := result.Subscription.GracePeriodEnd()
		grace = &graceAt
	}
	return output.StateChangedEvent{
		SubscriptionID:   result.Subscription.ID().String(),
		UserID:           result.Subscription.UserID().String(),
		WhatsAppNumber:   result.WhatsAppNumber,
		PlanCode:         result.Subscription.PlanCode().String(),
		PreviousState:    result.PreviousState.String(),
		NewState:         result.Subscription.InternalStatus().String(),
		TransitionReason: result.TransitionReason.String(),
		PeriodEnd:        result.Subscription.PeriodEnd(),
		GracePeriodEnd:   grace,
		OccurredAtValue:  occurredAt,
	}
}

// isStaleEvent retorna true quando o evento é anterior ao último evento aplicado (RF-25).
func isStaleEvent(sub *entities.Subscription, occurredAt time.Time) bool {
	if sub == nil {
		return false
	}
	return occurredAt.Before(sub.LastEventAt())
}

// applyCanonicalToSub mapeia o CanonicalEvent para a mutação correta na Subscription.
// Retorna (sub, applied, err). applied=false indica no-op.
func (p *ProcessBillingEventUseCase) applyCanonicalToSub(
	sub *entities.Subscription,
	canonical services.CanonicalEvent,
	userID identityentities.UserID,
	webhookID valueobjects.WebhookEventID,
	now time.Time,
) (*entities.Subscription, bool, valueobjects.TransitionReason, error) {
	period := services.PeriodChange{NewStart: canonical.PeriodStart, NewEnd: canonical.PeriodEnd}
	switch canonical.Type {
	case valueobjects.CanonicalEventPurchaseApproved:
		return p.applyPurchaseApproved(sub, canonical, userID, webhookID, now, period)
	case valueobjects.CanonicalEventRenewed:
		return applyWithPeriod(sub, valueobjects.TransitionReasonRenewed, func() error { return sub.Renew(now, period) })
	case valueobjects.CanonicalEventLate:
		return applySimple(sub, valueobjects.TransitionReasonLate, func() error { return sub.MarkPastDue(now) })
	case valueobjects.CanonicalEventCanceled:
		return applySimple(sub, valueobjects.TransitionReasonCanceled, func() error { return sub.Cancel(now) })
	case valueobjects.CanonicalEventRefunded, valueobjects.CanonicalEventChargeback:
		return p.applyRefund(sub, canonical, now)
	case valueobjects.CanonicalEventExpired:
		return applySimple(sub, valueobjects.TransitionReasonReconciliationSync, func() error { return sub.Expire(now) })
	default:
		return nil, false, valueobjects.TransitionReasonUnknown, fmt.Errorf("tipo de evento não mapeado: %s", canonical.Type.String())
	}
}

func (p *ProcessBillingEventUseCase) applyPurchaseApproved(
	sub *entities.Subscription,
	canonical services.CanonicalEvent,
	userID identityentities.UserID,
	webhookID valueobjects.WebhookEventID,
	now time.Time,
	period services.PeriodChange,
) (*entities.Subscription, bool, valueobjects.TransitionReason, error) {
	if sub == nil {
		created, err := p.createNewSubscription(canonical, userID, webhookID, now)
		if err != nil {
			return nil, false, valueobjects.TransitionReasonUnknown, err
		}
		return created, true, valueobjects.TransitionReasonPurchaseApproved, nil
	}
	// Reativação ou renovação de subscription existente (Active→Active via Renew é legal).
	if err := sub.Renew(now, period); err != nil {
		return nil, false, valueobjects.TransitionReasonUnknown, err
	}
	return sub, true, valueobjects.TransitionReasonPurchaseApproved, nil
}

func (p *ProcessBillingEventUseCase) applyRefund(
	sub *entities.Subscription,
	canonical services.CanonicalEvent,
	now time.Time,
) (*entities.Subscription, bool, valueobjects.TransitionReason, error) {
	if sub == nil {
		return nil, false, valueobjects.TransitionReasonUnknown, nil
	}
	amount, err := valueobjects.NewMoneyBRL(canonical.RefundAmountCents)
	if err != nil {
		return nil, false, valueobjects.TransitionReasonUnknown, err
	}
	reason := valueobjects.TransitionReasonRefunded
	if canonical.Type == valueobjects.CanonicalEventChargeback {
		reason = valueobjects.TransitionReasonChargebackReceived
	}
	if err := sub.Refund(now, amount, reason); err != nil {
		return nil, false, valueobjects.TransitionReasonUnknown, err
	}
	return sub, true, reason, nil
}

// applySimple aplica mutação sem mudança de período; retorna no-op se sub == nil.
func applySimple(sub *entities.Subscription, reason valueobjects.TransitionReason, mutate func() error) (*entities.Subscription, bool, valueobjects.TransitionReason, error) {
	if sub == nil {
		return nil, false, valueobjects.TransitionReasonUnknown, nil
	}
	if err := mutate(); err != nil {
		return nil, false, valueobjects.TransitionReasonUnknown, err
	}
	return sub, true, reason, nil
}

// applyWithPeriod aplica mutação com período; retorna no-op se sub == nil.
func applyWithPeriod(sub *entities.Subscription, reason valueobjects.TransitionReason, mutate func() error) (*entities.Subscription, bool, valueobjects.TransitionReason, error) {
	if sub == nil {
		return nil, false, valueobjects.TransitionReasonUnknown, nil
	}
	if err := mutate(); err != nil {
		return nil, false, valueobjects.TransitionReasonUnknown, err
	}
	return sub, true, reason, nil
}

func (p *ProcessBillingEventUseCase) createNewSubscription(
	canonical services.CanonicalEvent,
	userID identityentities.UserID,
	webhookID valueobjects.WebhookEventID,
	now time.Time,
) (*entities.Subscription, error) {
	subID, err := entities.NewSubscriptionID(p.idGenerator.NewID())
	if err != nil {
		return nil, fmt.Errorf("criar subscription id: %w", err)
	}
	extSubID, err := valueobjects.NewExternalSubscriptionID(canonical.ExternalSubscriptionID)
	if err != nil {
		return nil, fmt.Errorf("criar external subscription id: %w", err)
	}
	return entities.NewSubscription(entities.NewSubscriptionParams{
		ID:                 subID,
		UserID:             userID,
		Provider:           "kiwify",
		ExternalSubID:      extSubID,
		PlanCode:           canonical.PlanCode,
		InitialStatus:      valueobjects.SubscriptionStatusActive,
		PeriodStart:        canonical.PeriodStart,
		PeriodEnd:          canonical.PeriodEnd,
		LastEventAt:        canonical.OccurredAt,
		LastWebhookEventID: webhookID,
		CreatedAt:          now,
	})
}
