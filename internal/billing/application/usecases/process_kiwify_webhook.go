package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
)

const signatureStatusInvalid = "invalid"

type processSaleApproved interface {
	Execute(ctx context.Context, in input.ProcessSaleApprovedInput) error
}

type processSubscriptionRenewed interface {
	Execute(ctx context.Context, in input.ProcessSubscriptionRenewedInput) error
}

type processSubscriptionLate interface {
	Execute(ctx context.Context, in input.ProcessSubscriptionLateInput) error
}

type processSubscriptionCanceled interface {
	Execute(ctx context.Context, in input.ProcessSubscriptionCanceledInput) error
}

type processRefundOrChargeback interface {
	Execute(ctx context.Context, in input.ProcessRefundOrChargebackInput) error
}

type kiwifyWebhookPayload struct {
	OrderID          string `json:"order_id"`
	OrderRef         string `json:"order_ref"`
	OrderStatus      string `json:"order_status"`
	WebhookEventType string `json:"webhook_event_type"`
	SubscriptionID   string `json:"subscription_id"`

	AbandonedID     string `json:"id"`
	AbandonedStatus string `json:"status"`

	Product            productData       `json:"Product"`
	Customer           customerData      `json:"Customer"`
	Subscription       *subscriptionData `json:"Subscription"`
	TrackingParameters trackingData      `json:"TrackingParameters"`

	RefundedAt   *kiwifyTime `json:"refunded_at"`
	ApprovedDate kiwifyTime  `json:"approved_date"`
	UpdatedAt    kiwifyTime  `json:"updated_at"`
	CreatedAt    kiwifyTime  `json:"created_at"`
}

type productData struct {
	ProductID   string `json:"product_id"`
	ProductName string `json:"product_name"`
}

type customerData struct {
	Email  string `json:"email"`
	Mobile string `json:"mobile"`
	CPF    string `json:"CPF"`
}

type subscriptionData struct {
	StartDate   kiwifyTime `json:"start_date"`
	NextPayment kiwifyTime `json:"next_payment"`
	Status      string     `json:"status"`
}

type trackingData struct {
	SCK string `json:"sck"`
	S1  string `json:"s1"`
	Src string `json:"src"`
}

type kiwifyTime struct{ time.Time }

func (t *kiwifyTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		return nil
	}
	if parsed, err := time.Parse(time.RFC3339Nano, s); err == nil {
		t.Time = parsed.UTC()
		return nil
	}
	if parsed, err := time.Parse(time.RFC3339, s); err == nil {
		t.Time = parsed.UTC()
		return nil
	}
	brt := time.FixedZone("BRT", -3*60*60)
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02 15:04"} {
		if parsed, err := time.ParseInLocation(layout, s, brt); err == nil {
			t.Time = parsed.UTC()
			return nil
		}
	}
	return fmt.Errorf("kiwifyTime: cannot parse %q", s)
}

func (p kiwifyWebhookPayload) envelopeID() string {
	if p.OrderID != "" {
		return p.OrderID
	}
	return p.AbandonedID
}

func (p kiwifyWebhookPayload) eventType() string {
	if p.WebhookEventType != "" {
		return p.WebhookEventType
	}
	if p.AbandonedID != "" || p.AbandonedStatus == "abandoned" {
		return "abandoned_cart"
	}
	return ""
}

func (p kiwifyWebhookPayload) funnelToken() string {
	return extractFunnelToken(kiwifyTracking(p.TrackingParameters))
}

func (p kiwifyWebhookPayload) funnelCarrier() string {
	switch {
	case p.TrackingParameters.SCK != "":
		return "sck"
	case p.TrackingParameters.S1 != "":
		return "s1"
	case p.TrackingParameters.Src != "":
		return "src"
	default:
		return "none"
	}
}

func (p kiwifyWebhookPayload) approvedAtUTC() time.Time {
	if !p.ApprovedDate.IsZero() {
		return p.ApprovedDate.Time
	}
	if !p.UpdatedAt.IsZero() {
		return p.UpdatedAt.Time
	}
	if p.Subscription != nil && !p.Subscription.StartDate.IsZero() {
		return p.Subscription.StartDate.Time
	}
	return time.Time{}
}

func (p kiwifyWebhookPayload) renewalAtUTC() time.Time {
	if !p.UpdatedAt.IsZero() {
		return p.UpdatedAt.Time
	}
	if p.Subscription != nil && !p.Subscription.NextPayment.IsZero() {
		return p.Subscription.NextPayment.Time
	}
	return time.Time{}
}

func (p kiwifyWebhookPayload) cancellationAtUTC() time.Time {
	if !p.UpdatedAt.IsZero() {
		return p.UpdatedAt.Time
	}
	if p.Subscription != nil && !p.Subscription.StartDate.IsZero() {
		return p.Subscription.StartDate.Time
	}
	return time.Time{}
}

func (p kiwifyWebhookPayload) refundAtUTC() time.Time {
	if p.RefundedAt != nil && !p.RefundedAt.IsZero() {
		return p.RefundedAt.Time
	}
	return p.UpdatedAt.Time
}

type ProcessKiwifyWebhook struct {
	saleApproved    processSaleApproved
	subRenewed      processSubscriptionRenewed
	subLate         processSubscriptionLate
	subCanceled     processSubscriptionCanceled
	refundOrCharge  processRefundOrChargeback
	factory         interfaces.RepositoryFactory
	db              database.DBTX
	o11y            observability.Observability
	received        observability.Counter
	trackingCarrier observability.Counter
}

func NewProcessKiwifyWebhook(
	saleApproved processSaleApproved,
	subRenewed processSubscriptionRenewed,
	subLate processSubscriptionLate,
	subCanceled processSubscriptionCanceled,
	refundOrCharge processRefundOrChargeback,
	factory interfaces.RepositoryFactory,
	db database.DBTX,
	o11y observability.Observability,
) *ProcessKiwifyWebhook {
	received := o11y.Metrics().Counter(
		"billing_webhooks_received_total",
		"Total de webhooks Kiwify recebidos por status de assinatura",
		"1",
	)
	trackingCarrier := o11y.Metrics().Counter(
		"billing_kiwify_tracking_carrier_total",
		"Total de webhooks por carrier de funnel token (sck|s1|src|none)",
		"1",
	)
	return &ProcessKiwifyWebhook{
		saleApproved:    saleApproved,
		subRenewed:      subRenewed,
		subLate:         subLate,
		subCanceled:     subCanceled,
		refundOrCharge:  refundOrCharge,
		factory:         factory,
		db:              db,
		o11y:            o11y,
		received:        received,
		trackingCarrier: trackingCarrier,
	}
}

func (u *ProcessKiwifyWebhook) Execute(ctx context.Context, in input.ProcessKiwifyWebhookInput) error {
	ctx, span := u.o11y.Tracer().Start(ctx, "billing.usecase.process_kiwify_webhook")
	defer span.End()

	payload, err := u.parsePayload(in.RawBody)
	if err != nil {
		return err
	}

	u.auditEnvelope(ctx, payload, in.RawBody, in.SignatureStatus)
	u.received.Add(ctx, 1, observability.String("signature_status", in.SignatureStatus))
	if in.SignatureStatus == signatureStatusInvalid {
		u.o11y.Logger().Warn(ctx, "billing.webhook.signature_invalid",
			observability.String("envelope_id", payload.envelopeID()),
			observability.String("event_type", payload.eventType()),
		)
		return ErrInvalidSignature
	}

	carrier := payload.funnelCarrier()
	u.trackingCarrier.Add(ctx, 1, observability.String("carrier", carrier))
	if carrier == "s1" || carrier == "src" {
		u.o11y.Logger().Info(ctx, "kiwify.tracking.legacy_carrier_seen",
			observability.String("carrier", carrier),
			observability.String("envelope_id", payload.envelopeID()),
		)
	}

	if err := u.dispatch(ctx, payload); err != nil {
		span.RecordError(err)
		return err
	}
	return nil
}

func (u *ProcessKiwifyWebhook) parsePayload(raw []byte) (kiwifyWebhookPayload, error) {
	var payload kiwifyWebhookPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return kiwifyWebhookPayload{}, errors.Join(
			ErrInvalidWebhookPayload,
			fmt.Errorf("billing.usecase.process_kiwify_webhook parse payload: %w", err),
		)
	}
	return payload, nil
}

func (u *ProcessKiwifyWebhook) auditEnvelope(
	ctx context.Context,
	payload kiwifyWebhookPayload,
	raw []byte,
	signatureStatus string,
) {
	repo := u.factory.KiwifyEventRepository(u.db)
	envelopeID := payload.envelopeID()
	if err := repo.Persist(ctx, envelopeID, payload.eventType(), raw, signatureStatus); err != nil {
		u.o11y.Logger().Error(ctx, "billing.webhook.kiwify_events.persist_failed",
			observability.String("envelope_id", envelopeID),
			observability.Error(err),
		)
	}
}

func (u *ProcessKiwifyWebhook) dispatch(ctx context.Context, p kiwifyWebhookPayload) error {
	switch p.eventType() {
	case "billet_created", "pix_created", "order_rejected", "abandoned_cart":
		return nil
	case "order_approved":
		return u.saleApproved.Execute(ctx, input.ProcessSaleApprovedInput{
			EnvelopeID:         p.OrderID,
			SaleID:             p.OrderID,
			KiwifyProductID:    p.Product.ProductID,
			OrderID:            p.OrderID,
			FunnelToken:        p.funnelToken(),
			CustomerMobileE164: p.Customer.Mobile,
			CustomerEmail:      p.Customer.Email,
			OccurredAt:         p.approvedAtUTC(),
		})
	case "subscription_renewed":
		return u.subRenewed.Execute(ctx, input.ProcessSubscriptionRenewedInput{
			EnvelopeID:      p.OrderID,
			SaleID:          p.OrderID,
			KiwifyProductID: p.Product.ProductID,
			OrderID:         p.OrderID,
			KiwifySubID:     p.SubscriptionID,
			OccurredAt:      p.renewalAtUTC(),
		})
	case "subscription_late":
		return u.subLate.Execute(ctx, input.ProcessSubscriptionLateInput{
			EnvelopeID:  p.OrderID,
			SaleID:      p.OrderID,
			OrderID:     p.OrderID,
			KiwifySubID: p.SubscriptionID,
			OccurredAt:  p.renewalAtUTC(),
		})
	case "subscription_canceled":
		return u.subCanceled.Execute(ctx, input.ProcessSubscriptionCanceledInput{
			EnvelopeID:  p.OrderID,
			SaleID:      p.OrderID,
			OrderID:     p.OrderID,
			KiwifySubID: p.SubscriptionID,
			OccurredAt:  p.cancellationAtUTC(),
		})
	case "order_refunded", "chargeback":
		return u.refundOrCharge.Execute(ctx, input.ProcessRefundOrChargebackInput{
			EnvelopeID: p.OrderID,
			SaleID:     p.OrderID,
			OrderID:    p.OrderID,
			Trigger:    p.WebhookEventType,
			OccurredAt: p.refundAtUTC(),
		})
	default:
		return ErrUnknownTrigger
	}
}
