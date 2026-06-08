package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

type kiwifyEnvelope struct {
	ID      string          `json:"id"`
	Trigger string          `json:"trigger"`
	Data    json.RawMessage `json:"data"`
}

type saleData struct {
	ID           string       `json:"id"`
	OrderID      string       `json:"order_id"`
	ProductID    string       `json:"product_id"`
	RefundedAt   *time.Time   `json:"refunded_at"`
	Subscription *subData     `json:"subscription"`
	Customer     customerData `json:"customer"`
	Tracking     trackingData `json:"tracking"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

type subData struct {
	ID        string    `json:"id"`
	UpdatedAt time.Time `json:"updated_at"`
}

type customerData struct {
	Email  string `json:"email"`
	Mobile string `json:"mobile"`
}

type trackingData struct {
	SCK string `json:"sck"`
	S1  string `json:"s1"`
	Src string `json:"src"`
}

type ProcessKiwifyWebhook struct {
	saleApproved   processSaleApproved
	subRenewed     processSubscriptionRenewed
	subLate        processSubscriptionLate
	subCanceled    processSubscriptionCanceled
	refundOrCharge processRefundOrChargeback
	factory        interfaces.RepositoryFactory
	db             database.DBTX
	o11y           observability.Observability
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
	return &ProcessKiwifyWebhook{
		saleApproved:   saleApproved,
		subRenewed:     subRenewed,
		subLate:        subLate,
		subCanceled:    subCanceled,
		refundOrCharge: refundOrCharge,
		factory:        factory,
		db:             db,
		o11y:           o11y,
	}
}

func (u *ProcessKiwifyWebhook) Execute(ctx context.Context, in input.ProcessKiwifyWebhookInput) error {
	ctx, span := u.o11y.Tracer().Start(ctx, "billing.usecase.process_kiwify_webhook")
	defer span.End()

	envelope, err := u.parseEnvelope(in.RawBody)
	if err != nil {
		return err
	}

	u.auditEnvelope(ctx, envelope, in.RawBody, in.SignatureStatus)
	if in.SignatureStatus == signatureStatusInvalid {
		return ErrInvalidSignature
	}

	if err := u.dispatch(ctx, envelope); err != nil {
		span.RecordError(err)
		return err
	}
	return nil
}

func (u *ProcessKiwifyWebhook) parseEnvelope(raw []byte) (kiwifyEnvelope, error) {
	var envelope kiwifyEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return kiwifyEnvelope{}, errors.Join(
			ErrInvalidWebhookPayload,
			fmt.Errorf("billing.usecase.process_kiwify_webhook parse envelope: %w", err),
		)
	}
	return envelope, nil
}

func (u *ProcessKiwifyWebhook) auditEnvelope(
	ctx context.Context,
	envelope kiwifyEnvelope,
	raw []byte,
	signatureStatus string,
) {
	repo := u.factory.KiwifyEventRepository(u.db)
	if err := repo.Persist(ctx, envelope.ID, envelope.Trigger, raw, signatureStatus); err != nil {
		u.o11y.Logger().Error(ctx, "billing.webhook.kiwify_events.persist_failed",
			observability.String("envelope_id", envelope.ID),
			observability.Error(err),
		)
	}
}

func (u *ProcessKiwifyWebhook) dispatch(ctx context.Context, envelope kiwifyEnvelope) error {
	var sale saleData
	if err := json.Unmarshal(envelope.Data, &sale); err != nil {
		return ErrUnknownTrigger
	}

	switch envelope.Trigger {
	case "compra_aprovada":
		return u.saleApproved.Execute(ctx, input.ProcessSaleApprovedInput{
			EnvelopeID:         envelope.ID,
			SaleID:             sale.ID,
			KiwifyProductID:    sale.ProductID,
			OrderID:            sale.OrderID,
			FunnelToken:        u.extractFunnelToken(sale.Tracking),
			CustomerMobileE164: sale.Customer.Mobile,
			CustomerEmail:      sale.Customer.Email,
			OccurredAt:         sale.UpdatedAt,
		})
	case "subscription_renewed":
		subID, subUpdatedAt := u.extractSub(sale.Subscription)
		return u.subRenewed.Execute(ctx, input.ProcessSubscriptionRenewedInput{
			EnvelopeID:      envelope.ID,
			SaleID:          sale.ID,
			KiwifyProductID: sale.ProductID,
			OrderID:         sale.OrderID,
			KiwifySubID:     subID,
			OccurredAt:      subUpdatedAt,
		})
	case "subscription_late":
		subID, subUpdatedAt := u.extractSub(sale.Subscription)
		return u.subLate.Execute(ctx, input.ProcessSubscriptionLateInput{
			EnvelopeID:  envelope.ID,
			SaleID:      sale.ID,
			OrderID:     sale.OrderID,
			KiwifySubID: subID,
			OccurredAt:  subUpdatedAt,
		})
	case "subscription_canceled":
		subID, subUpdatedAt := u.extractSub(sale.Subscription)
		return u.subCanceled.Execute(ctx, input.ProcessSubscriptionCanceledInput{
			EnvelopeID:  envelope.ID,
			SaleID:      sale.ID,
			OrderID:     sale.OrderID,
			KiwifySubID: subID,
			OccurredAt:  subUpdatedAt,
		})
	case "compra_reembolsada", "chargeback":
		return u.refundOrCharge.Execute(ctx, input.ProcessRefundOrChargebackInput{
			EnvelopeID: envelope.ID,
			SaleID:     sale.ID,
			OrderID:    sale.OrderID,
			Trigger:    envelope.Trigger,
			OccurredAt: u.refundOccurredAt(sale),
		})
	default:
		return ErrUnknownTrigger
	}
}

func (u *ProcessKiwifyWebhook) extractFunnelToken(tracking trackingData) string {
	return extractFunnelToken(kiwifyTracking(tracking))
}

func (u *ProcessKiwifyWebhook) extractSub(sub *subData) (string, time.Time) {
	if sub == nil {
		return "", time.Time{}
	}
	return sub.ID, sub.UpdatedAt
}

func (u *ProcessKiwifyWebhook) refundOccurredAt(sale saleData) time.Time {
	if sale.RefundedAt != nil {
		return *sale.RefundedAt
	}
	return sale.UpdatedAt
}
