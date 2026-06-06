package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/http/server/middleware"
)

type processKiwifyWebhookUseCase interface {
	Execute(ctx context.Context, in input.ProcessKiwifyWebhookInput) error
}

type webhookResponse struct {
	Received bool `json:"received"`
}

type KiwifyWebhookHandler struct {
	usecase processKiwifyWebhookUseCase
	o11y    observability.Observability
}

func NewKiwifyWebhookHandler(
	uc processKiwifyWebhookUseCase,
	o11y observability.Observability,
) *KiwifyWebhookHandler {
	return &KiwifyWebhookHandler{
		usecase: uc,
		o11y:    o11y,
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

	if err := h.usecase.Execute(ctx, input.ProcessKiwifyWebhookInput{
		RawBody:         raw,
		SignatureStatus: middleware.SignatureStatusFromContext(r),
	}); err != nil {
		span.RecordError(err)
		switch {
		case errors.Is(err, usecases.ErrInvalidWebhookPayload):
			responses.ErrorWithDetails(w, http.StatusUnprocessableEntity, "invalid payload",
				map[string]string{"code": "invalid_json"})
		case errors.Is(err, usecases.ErrInvalidSignature):
			responses.Error(w, http.StatusUnauthorized, "invalid signature")
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
				observability.Error(err),
			)
			responses.Error(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	responses.JSON(w, http.StatusAccepted, webhookResponse{Received: true})
}
