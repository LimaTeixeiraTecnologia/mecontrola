package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
)

type ActivateTelegramByTokenUseCase interface {
	Execute(ctx context.Context, in usecases.ActivateTelegramByTokenInput) (usecases.ActivateTelegramResult, error)
}

type TelegramProcessOnboardingUseCase interface {
	Execute(ctx context.Context, in usecases.ProcessOnboardingMessageInput) (usecases.ProcessOnboardingMessageResult, error)
}

type TelegramMessageProcessor struct {
	activateUseCase ActivateTelegramByTokenUseCase
	processUseCase  TelegramProcessOnboardingUseCase
	messages        map[string]string
	o11y            observability.Observability
	inbound         observability.Counter
}

func NewTelegramMessageProcessor(
	activateUseCase ActivateTelegramByTokenUseCase,
	processUseCase TelegramProcessOnboardingUseCase,
	messages map[string]string,
	o11y observability.Observability,
) *TelegramMessageProcessor {
	return &TelegramMessageProcessor{
		activateUseCase: activateUseCase,
		processUseCase:  processUseCase,
		messages:        messages,
		o11y:            o11y,
		inbound: o11y.Metrics().Counter(
			"telegram_onboarding_inbound_total",
			"Total de mensagens de onboarding recebidas no Telegram",
			"1",
		),
	}
}

func (p *TelegramMessageProcessor) ProcessConversation(ctx context.Context, userID uuid.UUID, text, messageID string) (string, error) {
	if p.processUseCase == nil {
		return "", application.ErrOnboardingAlreadyActive
	}
	result, err := p.processUseCase.Execute(ctx, usecases.ProcessOnboardingMessageInput{
		UserID:    userID,
		Channel:   entities.OnboardingChannelTelegram,
		MessageID: messageID,
		Text:      text,
	})
	if err != nil {
		if errors.Is(err, appinterfaces.ErrOnboardingSessionNotFound) {
			return "", appinterfaces.ErrOnboardingSessionNotFound
		}
		slog.WarnContext(ctx, "telegram.processor.conversation_failed",
			"user_id", userID.String(),
			"error", err.Error(),
		)
		return "", fmt.Errorf("onboarding.telegram.processor: process conversation: %w", err)
	}
	if result.Outcome == usecases.ProcessOnboardingOutcomeNoOp {
		return "", application.ErrOnboardingAlreadyActive
	}
	return result.Reply, nil
}

func (p *TelegramMessageProcessor) HandleActivation(ctx context.Context, telegramUserID int64, token string) (string, error) {
	p.inbound.Add(ctx, 1, observability.String("kind", "activation_cmd"))

	res, err := p.activateUseCase.Execute(ctx, usecases.ActivateTelegramByTokenInput{
		Token:          token,
		TelegramUserID: telegramUserID,
	})
	if err != nil {
		slog.ErrorContext(ctx, "telegram.processor.activation_failed",
			"telegram_user_id_masked", maskTelegramUserID(telegramUserID),
			"error", err.Error(),
		)
		return p.msg("system_unavailable_retry"), fmt.Errorf("onboarding.telegram.processor: activate: %w", err)
	}

	return p.msg(activateOutcomeToMessageKey(res.Outcome)), nil
}

func (p *TelegramMessageProcessor) HandleFallback(ctx context.Context, _ int64) (string, error) {
	p.inbound.Add(ctx, 1, observability.String("kind", "fallback_candidate"))
	return p.msg("please_use_ativar_command"), nil
}

func (p *TelegramMessageProcessor) msg(key string) string {
	if v, ok := p.messages[key]; ok {
		return v
	}
	return key
}

func maskTelegramUserID(id int64) string {
	raw := fmt.Sprintf("%d", id)
	if len(raw) <= 4 {
		return "****"
	}
	return "***" + raw[len(raw)-4:]
}

func activateOutcomeToMessageKey(outcome usecases.ActivateTelegramOutcome) string {
	switch outcome {
	case usecases.ActivateTelegramOutcomeLinked:
		return "welcome_activated"
	case usecases.ActivateTelegramOutcomeAlreadyLinked:
		return "already_active"
	case usecases.ActivateTelegramOutcomeRequiresWhatsAppActivation:
		return "requires_whatsapp_activation"
	case usecases.ActivateTelegramOutcomeReusedOtherAccount:
		return "code_already_used_other_account"
	case usecases.ActivateTelegramOutcomeNotYetPaid:
		return "payment_still_processing_retry"
	case usecases.ActivateTelegramOutcomeExpired:
		return "code_expired_contact_support"
	case usecases.ActivateTelegramOutcomeNotFound:
		return "code_invalid_check_again"
	default:
		return "system_unavailable_retry"
	}
}
