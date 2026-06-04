package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/http/client/kiwify"
)

const webhookBodyLimitBytes = 1 * 1024 * 1024 // 1 MiB

var webhookTracer = otel.Tracer("billing.webhook")

// ingestWebhookExecutor é o contrato mínimo do IngestKiwifyWebhookUseCase consumido pelo handler.
type ingestWebhookExecutor interface {
	Execute(ctx context.Context, in input.IngestWebhookInput) (output.IngestWebhookResult, error)
}

// KiwifyWebhookHandler processa requisições POST /webhooks/kiwify.
// Traduz erros do adaptador Kiwify para status HTTP apropriados (RF-01, RF-06, RF-09).
type KiwifyWebhookHandler struct {
	useCase ingestWebhookExecutor
	logger  *slog.Logger
	header  string
}

// NewKiwifyWebhookHandler cria um KiwifyWebhookHandler com as dependências obrigatórias.
func NewKiwifyWebhookHandler(
	useCase ingestWebhookExecutor,
	logger *slog.Logger,
	tokenHeader ...string,
) *KiwifyWebhookHandler {
	header := "X-Kiwify-Webhook-Token"
	if len(tokenHeader) > 0 && strings.TrimSpace(tokenHeader[0]) != "" {
		header = tokenHeader[0]
	}
	return &KiwifyWebhookHandler{
		useCase: useCase,
		logger:  logger,
		header:  header,
	}
}

func (h *KiwifyWebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, span := webhookTracer.Start(r.Context(), "billing.webhook.ingress")
	defer span.End()

	body, err := io.ReadAll(io.LimitReader(r.Body, webhookBodyLimitBytes))
	if err != nil {
		span.SetStatus(codes.Error, "leitura body")
		h.logger.ErrorContext(ctx, "billing webhook: leitura body", "error", err)
		writeWebhookJSON(w, http.StatusInternalServerError)
		return
	}

	result, err := h.useCase.Execute(ctx, input.IngestWebhookInput{
		RawBody:             body,
		Headers:             extractHeaders(r),
		SignatureHeaderName: h.header,
		ReceivedAt:          time.Now().UTC(),
	})
	if err != nil {
		h.handleError(ctx, w, r, span, err)
		return
	}

	if result.Duplicate {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"received":  true,
		"duplicate": false,
	})
}

func (h *KiwifyWebhookHandler) handleError(
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
	span oteltrace.Span,
	err error,
) {
	switch {
	case errors.Is(err, kiwify.ErrInvalidSignature), errors.Is(err, kiwify.ErrMissingSignature):
		span.SetStatus(codes.Error, "assinatura inválida")
		writeWebhookJSON(w, http.StatusUnauthorized)
	case errors.Is(err, kiwify.ErrPayloadDecode),
		errors.Is(err, valueobjects.ErrEmptyPayload),
		errors.Is(err, valueobjects.ErrMalformedPayload):
		span.SetStatus(codes.Error, "payload inválido")
		writeWebhookJSON(w, http.StatusBadRequest)
	default:
		span.SetStatus(codes.Error, "erro interno")
		h.logger.ErrorContext(ctx, "billing webhook: erro interno",
			"correlation_id", r.Header.Get("X-Request-ID"),
			"error", err,
		)
		writeWebhookJSON(w, http.StatusInternalServerError)
	}
}

func writeWebhookJSON(w http.ResponseWriter, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
}

// extractHeaders copia os headers da requisição excluindo Authorization e Cookie (RF-08).
func extractHeaders(r *http.Request) map[string]string {
	headers := make(map[string]string, len(r.Header))
	for k, vals := range r.Header {
		if strings.EqualFold(k, "authorization") || strings.EqualFold(k, "cookie") {
			continue
		}
		if len(vals) > 0 {
			headers[k] = vals[0]
		}
	}
	return headers
}
