package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/http/server/middleware"
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

type dbManager interface {
	DBTX(ctx context.Context) database.DBTX
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
	Tracking     trackingData `json:"tracking"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

type subData struct {
	ID        string    `json:"id"`
	UpdatedAt time.Time `json:"updated_at"`
}

type trackingData struct {
	S1  string `json:"s1"`
	Src string `json:"src"`
}

type webhookResponse struct {
	Received bool `json:"received"`
}

type KiwifyWebhookHandler struct {
	saleApproved   processSaleApproved
	subRenewed     processSubscriptionRenewed
	subLate        processSubscriptionLate
	subCanceled    processSubscriptionCanceled
	refundOrCharge processRefundOrChargeback
	factory        interfaces.RepositoryFactory
	mgr            dbManager
	o11y           observability.Observability
}

func NewKiwifyWebhookHandler(
	saleApproved processSaleApproved,
	subRenewed processSubscriptionRenewed,
	subLate processSubscriptionLate,
	subCanceled processSubscriptionCanceled,
	refundOrCharge processRefundOrChargeback,
	factory interfaces.RepositoryFactory,
	mgr dbManager,
	o11y observability.Observability,
) *KiwifyWebhookHandler {
	return &KiwifyWebhookHandler{
		saleApproved:   saleApproved,
		subRenewed:     subRenewed,
		subLate:        subLate,
		subCanceled:    subCanceled,
		refundOrCharge: refundOrCharge,
		factory:        factory,
		mgr:            mgr,
		o11y:           o11y,
	}
}

func (h *KiwifyWebhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "billing.handler.kiwify_webhook")
	defer span.End()

	ct := r.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		responses.Error(w, http.StatusUnsupportedMediaType, "unsupported media type")
		return
	}

	raw, ok := middleware.RawBodyFromContext(r)
	if !ok {
		responses.Error(w, http.StatusInternalServerError, "raw body unavailable")
		return
	}

	var env kiwifyEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		responses.ErrorWithDetails(w, http.StatusUnprocessableEntity, "invalid payload",
			map[string]string{"code": "invalid_json"})
		return
	}

	sigStatus := middleware.SignatureStatusFromContext(r)

	kiwifyRepo := h.factory.KiwifyEventRepository(h.mgr.DBTX(ctx))
	if persistErr := kiwifyRepo.Persist(ctx, env.ID, env.Trigger, raw, sigStatus); persistErr != nil {
		h.o11y.Logger().Error(ctx, "billing.webhook.kiwify_events.persist_failed",
			observability.String("envelope_id", env.ID),
			observability.Error(persistErr),
		)
	}

	if err := h.dispatch(ctx, env); err != nil {
		span.RecordError(err)
		switch {
		case errors.Is(err, usecases.ErrEventAlreadyProcessed):
			responses.JSON(w, http.StatusAccepted, webhookResponse{Received: true})
		case errors.Is(err, usecases.ErrEventSuperseded):
			responses.JSON(w, http.StatusAccepted, webhookResponse{Received: true})
		case errors.Is(err, usecases.ErrFunnelTokenMissing):
			responses.ErrorWithDetails(w, http.StatusUnprocessableEntity, "funnel token missing",
				map[string]string{"code": "funnel_token_missing"})
		case errors.Is(err, usecases.ErrUnknownTrigger):
			responses.ErrorWithDetails(w, http.StatusUnprocessableEntity, "unknown trigger",
				map[string]string{"code": "unknown_trigger"})
		default:
			h.o11y.Logger().Error(ctx, "billing.webhook.dispatch_failed",
				observability.String("envelope_id", env.ID),
				observability.String("trigger", env.Trigger),
				observability.Error(err),
			)
			responses.Error(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	responses.JSON(w, http.StatusAccepted, webhookResponse{Received: true})
}

func (h *KiwifyWebhookHandler) dispatch(ctx context.Context, env kiwifyEnvelope) error {
	var sale saleData
	if err := json.Unmarshal(env.Data, &sale); err != nil {
		return usecases.ErrUnknownTrigger
	}

	switch env.Trigger {
	case "compra_aprovada":
		return h.saleApproved.Execute(ctx, input.ProcessSaleApprovedInput{
			EnvelopeID:      env.ID,
			SaleID:          sale.ID,
			KiwifyProductID: sale.ProductID,
			OrderID:         sale.OrderID,
			FunnelToken:     extractFunnelToken(sale.Tracking),
			OccurredAt:      sale.UpdatedAt,
		})

	case "subscription_renewed":
		subID, subUpdatedAt := extractSub(sale.Subscription)
		return h.subRenewed.Execute(ctx, input.ProcessSubscriptionRenewedInput{
			EnvelopeID:      env.ID,
			SaleID:          sale.ID,
			KiwifyProductID: sale.ProductID,
			OrderID:         sale.OrderID,
			KiwifySubID:     subID,
			OccurredAt:      subUpdatedAt,
		})

	case "subscription_late":
		subID, subUpdatedAt := extractSub(sale.Subscription)
		return h.subLate.Execute(ctx, input.ProcessSubscriptionLateInput{
			EnvelopeID:  env.ID,
			SaleID:      sale.ID,
			OrderID:     sale.OrderID,
			KiwifySubID: subID,
			OccurredAt:  subUpdatedAt,
		})

	case "subscription_canceled":
		subID, subUpdatedAt := extractSub(sale.Subscription)
		return h.subCanceled.Execute(ctx, input.ProcessSubscriptionCanceledInput{
			EnvelopeID:  env.ID,
			SaleID:      sale.ID,
			OrderID:     sale.OrderID,
			KiwifySubID: subID,
			OccurredAt:  subUpdatedAt,
		})

	case "compra_reembolsada", "chargeback":
		occurredAt := sale.UpdatedAt
		if sale.RefundedAt != nil {
			occurredAt = *sale.RefundedAt
		}
		return h.refundOrCharge.Execute(ctx, input.ProcessRefundOrChargebackInput{
			EnvelopeID: env.ID,
			SaleID:     sale.ID,
			OrderID:    sale.OrderID,
			Trigger:    env.Trigger,
			OccurredAt: occurredAt,
		})

	default:
		return usecases.ErrUnknownTrigger
	}
}

func extractFunnelToken(t trackingData) string {
	if t.S1 != "" {
		return t.S1
	}
	return t.Src
}

func extractSub(sub *subData) (string, time.Time) {
	if sub == nil {
		return "", time.Time{}
	}
	return sub.ID, sub.UpdatedAt
}
