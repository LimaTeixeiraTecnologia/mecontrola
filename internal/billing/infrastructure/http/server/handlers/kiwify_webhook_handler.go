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

type webhookErrorMapping struct {
	target error
	body   func(http.ResponseWriter)
}

func respondAccepted(w http.ResponseWriter) {
	responses.JSON(w, http.StatusAccepted, webhookResponse{Received: true})
}

func respondUnprocessable(message, code string) func(http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		responses.ErrorWithDetails(w, http.StatusUnprocessableEntity, message, map[string]string{"code": code})
	}
}

var webhookErrorTable = []webhookErrorMapping{
	{target: usecases.ErrInvalidWebhookPayload, body: respondUnprocessable("invalid payload", "invalid_json")},
	{target: usecases.ErrInvalidSignature, body: func(w http.ResponseWriter) { responses.Error(w, http.StatusUnauthorized, "invalid signature") }},
	{target: usecases.ErrEventAlreadyProcessed, body: respondAccepted},
	{target: usecases.ErrEventSuperseded, body: respondAccepted},
	{target: usecases.ErrFunnelTokenMissing, body: respondUnprocessable("funnel token missing", "funnel_token_missing")},
	{target: usecases.ErrKiwifySubscriptionIDInvalid, body: respondUnprocessable("invalid kiwify subscription id", "invalid_kiwify_subscription_id")},
	{target: usecases.ErrUnknownTrigger, body: respondUnprocessable("unknown trigger", "unknown_trigger")},
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
		for _, m := range webhookErrorTable {
			if errors.Is(err, m.target) {
				m.body(w)
				return
			}
		}
		h.o11y.Logger().Error(ctx, "billing.webhook.dispatch_failed", observability.Error(err))
		responses.Error(w, http.StatusInternalServerError, "internal error")
		return
	}

	responses.JSON(w, http.StatusAccepted, webhookResponse{Received: true})
}
