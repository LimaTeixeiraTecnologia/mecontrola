package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	identityvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/input"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

var ativarRegex = regexp.MustCompile(`(?i)^\s*ATIVAR\s+([A-Za-z0-9_\-]{40,45})\s*$`)

type consumeMagicTokenUseCase interface {
	Execute(ctx context.Context, in input.ConsumeMagicTokenInput) (usecases.ConsumeResult, error)
}

type tryFallbackActivationUseCase interface {
	Execute(ctx context.Context, fromE164 string) (usecases.FallbackResult, error)
}

type whatsAppGateway interface {
	SendTextMessage(ctx context.Context, toE164, text string) error
}

type WhatsAppInboundHandler struct {
	consumeUseCase     consumeMagicTokenUseCase
	fallbackUseCase    tryFallbackActivationUseCase
	waGateway          whatsAppGateway
	factory            appinterfaces.RepositoryFactory
	db                 database.DBTX
	messages           map[string]string
	o11y               observability.Observability
	inboundMessages    observability.Counter
	duplicateMessages  observability.Counter
	confirmationFailed observability.Counter
}

func NewWhatsAppInboundHandler(
	consumeUseCase consumeMagicTokenUseCase,
	fallbackUseCase tryFallbackActivationUseCase,
	waGateway whatsAppGateway,
	factory appinterfaces.RepositoryFactory,
	db database.DBTX,
	messages map[string]string,
	o11y observability.Observability,
) *WhatsAppInboundHandler {
	inboundMessages := o11y.Metrics().Counter(
		"meta_inbound_messages_total",
		"Total de mensagens inbound recebidas do WhatsApp",
		"1",
	)
	duplicateMessages := o11y.Metrics().Counter(
		"meta_duplicate_messages_total",
		"Total de mensagens duplicadas detectadas por WAMID",
		"1",
	)
	confirmationFailed := o11y.Metrics().Counter(
		"onboarding_confirmation_failed_total",
		"Total de falhas ao enviar confirmacao de ativacao ao cliente",
		"1",
	)
	return &WhatsAppInboundHandler{
		consumeUseCase:     consumeUseCase,
		fallbackUseCase:    fallbackUseCase,
		waGateway:          waGateway,
		factory:            factory,
		db:                 db,
		messages:           messages,
		o11y:               o11y,
		inboundMessages:    inboundMessages,
		duplicateMessages:  duplicateMessages,
		confirmationFailed: confirmationFailed,
	}
}

func (h *WhatsAppInboundHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "onboarding.handler.whatsapp_inbound")
	defer span.End()

	var payload metaWebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	msg, ok := extractFirstMessage(payload)
	if !ok {
		w.WriteHeader(http.StatusOK)
		return
	}

	msgRepo := h.factory.MetaMessageRepository(h.db)
	inserted, err := msgRepo.InsertIfAbsent(ctx, msg.WAMID)
	if err != nil {
		slog.WarnContext(ctx, "onboarding.inbound.wamid_check_failed",
			"wamid", msg.WAMID,
			"error", err.Error(),
		)
		w.WriteHeader(http.StatusOK)
		return
	}
	if !inserted {
		h.duplicateMessages.Add(ctx, 1)
		slog.InfoContext(ctx, "onboarding.inbound.duplicate_wamid", "wamid", msg.WAMID)
		w.WriteHeader(http.StatusOK)
		return
	}

	from, err := identityvo.NewWhatsAppNumber(msg.From)
	if err != nil {
		slog.InfoContext(ctx, "onboarding.inbound.invalid_country",
			"from", maskMobileHandler(msg.From),
		)
		h.sendMessage(ctx, msg.From, h.msg("invalid_country"))
		w.WriteHeader(http.StatusOK)
		return
	}
	msg.From = from.String()

	h.dispatch(ctx, msg, w)
}

func (h *WhatsAppInboundHandler) dispatch(ctx context.Context, msg parsedInboundMessage, w http.ResponseWriter) {
	matches := ativarRegex.FindStringSubmatch(strings.TrimSpace(msg.Text))
	if matches != nil {
		h.inboundMessages.Add(ctx, 1, observability.String("kind", "activation_cmd"))
		h.handleAtivar(ctx, msg.From, matches[1], w)
		return
	}

	h.inboundMessages.Add(ctx, 1, observability.String("kind", "fallback_candidate"))
	h.handleFallback(ctx, msg.From, w)
}

func (h *WhatsAppInboundHandler) handleAtivar(ctx context.Context, fromE164, token string, w http.ResponseWriter) {
	result, err := h.consumeUseCase.Execute(ctx, input.ConsumeMagicTokenInput{
		Token:          token,
		FromE164:       fromE164,
		ActivationPath: valueobjects.ActivationPathDirect,
	})
	if err != nil {
		slog.ErrorContext(ctx, "onboarding.inbound.consume_failed",
			"from", maskMobileHandler(fromE164),
			"error", err.Error(),
		)
		h.sendMessage(ctx, fromE164, h.msg("system_unavailable_retry"))
		w.WriteHeader(http.StatusOK)
		return
	}

	replyKey := consumeOutcomeToMessageKey(result.Outcome)
	h.sendMessage(ctx, fromE164, h.msg(replyKey))
	w.WriteHeader(http.StatusOK)
}

func (h *WhatsAppInboundHandler) handleFallback(ctx context.Context, fromE164 string, w http.ResponseWriter) {
	result, err := h.fallbackUseCase.Execute(ctx, fromE164)
	if err != nil {
		slog.WarnContext(ctx, "onboarding.inbound.fallback_failed",
			"from", maskMobileHandler(fromE164),
			"error", err.Error(),
		)
		w.WriteHeader(http.StatusOK)
		return
	}

	switch result.Outcome {
	case usecases.FallbackOutcomeActivated:
		h.sendMessage(ctx, fromE164, h.msg("welcome_activated"))
	case usecases.FallbackOutcomeOutreachRequired:
		h.sendMessage(ctx, fromE164, h.msg("please_use_ativar_command"))
	}
	w.WriteHeader(http.StatusOK)
}

func (h *WhatsAppInboundHandler) sendMessage(ctx context.Context, toE164, text string) {
	if err := h.waGateway.SendTextMessage(ctx, toE164, text); err != nil {
		h.confirmationFailed.Add(ctx, 1, observability.String("reason", "send_error"))
		slog.WarnContext(ctx, "onboarding.inbound.send_failed",
			"to", maskMobileHandler(toE164),
			"error", err.Error(),
		)
	}
}

func (h *WhatsAppInboundHandler) msg(key string) string {
	if v, ok := h.messages[key]; ok {
		return v
	}
	return key
}

func consumeOutcomeToMessageKey(outcome usecases.ConsumeOutcome) string {
	switch outcome {
	case usecases.ConsumeOutcomeActivated:
		return "welcome_activated"
	case usecases.ConsumeOutcomeAlreadyActive:
		return "already_active"
	case usecases.ConsumeOutcomeReuseOtherAccount:
		return "code_already_used_other_account"
	case usecases.ConsumeOutcomeNotYetPaid:
		return "payment_still_processing_retry"
	case usecases.ConsumeOutcomeExpired:
		return "code_expired_contact_support"
	case usecases.ConsumeOutcomeNotFound:
		return "code_invalid_check_again"
	default:
		return "system_unavailable_retry"
	}
}

func maskMobileHandler(mobile string) string {
	if len(mobile) < 4 {
		return "****"
	}
	return mobile[:3] + "****" + mobile[len(mobile)-4:]
}
