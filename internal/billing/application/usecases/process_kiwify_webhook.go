package usecases

import (
	"context"
	"errors"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases/kiwifypayload"
)

const (
	signatureStatusInvalid = "invalid"
	signatureStatusRotated = "rotated"
)

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

type triggerHandler func(ctx context.Context, p kiwifypayload.Payload) error

type ProcessKiwifyWebhook struct {
	factory          interfaces.RepositoryFactory
	db               database.DBTX
	o11y             observability.Observability
	received         observability.Counter
	trackingCarrier  observability.Counter
	signatureRotated observability.Counter
	handlers         map[kiwifypayload.Trigger]triggerHandler
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
	signatureRotated := o11y.Metrics().Counter(
		"billing_webhook_signature_rotated_total",
		"Total de webhooks aceitos via secret rotacional (KIWIFY_WEBHOOK_SECRET_NEXT)",
		"1",
	)

	uc := &ProcessKiwifyWebhook{
		factory:          factory,
		db:               db,
		o11y:             o11y,
		received:         received,
		trackingCarrier:  trackingCarrier,
		signatureRotated: signatureRotated,
	}
	uc.handlers = map[kiwifypayload.Trigger]triggerHandler{
		kiwifypayload.TriggerBilletCreated: noopTrigger,
		kiwifypayload.TriggerPixCreated:    noopTrigger,
		kiwifypayload.TriggerOrderRejected: noopTrigger,
		kiwifypayload.TriggerAbandonedCart: noopTrigger,
		kiwifypayload.TriggerOrderApproved: func(ctx context.Context, p kiwifypayload.Payload) error {
			return saleApproved.Execute(ctx, kiwifypayload.ToSaleApprovedInput(p))
		},
		kiwifypayload.TriggerSubscriptionRenewed: func(ctx context.Context, p kiwifypayload.Payload) error {
			return subRenewed.Execute(ctx, kiwifypayload.ToSubscriptionRenewedInput(p))
		},
		kiwifypayload.TriggerSubscriptionLate: func(ctx context.Context, p kiwifypayload.Payload) error {
			return subLate.Execute(ctx, kiwifypayload.ToSubscriptionLateInput(p))
		},
		kiwifypayload.TriggerSubscriptionCanceled: func(ctx context.Context, p kiwifypayload.Payload) error {
			return subCanceled.Execute(ctx, kiwifypayload.ToSubscriptionCanceledInput(p))
		},
		kiwifypayload.TriggerOrderRefunded: func(ctx context.Context, p kiwifypayload.Payload) error {
			return refundOrCharge.Execute(ctx, kiwifypayload.ToRefundOrChargebackInput(p))
		},
		kiwifypayload.TriggerChargeback: func(ctx context.Context, p kiwifypayload.Payload) error {
			return refundOrCharge.Execute(ctx, kiwifypayload.ToRefundOrChargebackInput(p))
		},
	}
	return uc
}

func noopTrigger(_ context.Context, _ kiwifypayload.Payload) error { return nil }

func (u *ProcessKiwifyWebhook) Execute(ctx context.Context, in input.ProcessKiwifyWebhookInput) error {
	ctx, span := u.o11y.Tracer().Start(ctx, "billing.usecase.process_kiwify_webhook")
	defer span.End()

	payload, err := kiwifypayload.Decode(in.RawBody)
	if err != nil {
		return errors.Join(
			ErrInvalidWebhookPayload,
			fmt.Errorf("billing.usecase.process_kiwify_webhook decode payload: %w", err),
		)
	}

	trigger := kiwifypayload.Classify(payload)
	envelopeID := payload.EnvelopeID()

	u.auditEnvelope(ctx, envelopeID, string(trigger), in.RawBody, in.SignatureStatus)
	u.received.Add(ctx, 1, observability.String("signature_status", in.SignatureStatus))
	if in.SignatureStatus == signatureStatusRotated {
		u.signatureRotated.Add(ctx, 1)
		u.o11y.Logger().Warn(ctx, "billing.webhook.signature_rotated",
			observability.String("envelope_id", envelopeID),
		)
	}
	if in.SignatureStatus == signatureStatusInvalid {
		u.o11y.Logger().Warn(ctx, "billing.webhook.signature_invalid",
			observability.String("envelope_id", envelopeID),
			observability.String("event_type", string(trigger)),
		)
		return ErrInvalidSignature
	}

	_, carrier := kiwifypayload.ExtractFunnel(payload)
	u.trackingCarrier.Add(ctx, 1, observability.String("carrier", carrier))
	if carrier == "s1" || carrier == "src" {
		u.o11y.Logger().Info(ctx, "kiwify.tracking.legacy_carrier_seen",
			observability.String("carrier", carrier),
			observability.String("envelope_id", envelopeID),
		)
	}

	handler, ok := u.handlers[trigger]
	if !ok {
		return ErrUnknownTrigger
	}
	if err := handler(ctx, payload); err != nil {
		span.RecordError(err)
		return err
	}
	return nil
}

func (u *ProcessKiwifyWebhook) auditEnvelope(
	ctx context.Context,
	envelopeID, eventType string,
	raw []byte,
	signatureStatus string,
) {
	repo := u.factory.KiwifyEventRepository(u.db)
	if err := repo.Persist(ctx, envelopeID, eventType, raw, signatureStatus); err != nil {
		u.o11y.Logger().Error(ctx, "billing.webhook.kiwify_events.persist_failed",
			observability.String("envelope_id", envelopeID),
			observability.Error(err),
		)
	}
}
