package services

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	identityvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/payload"
)

type ConsumeMagicTokenUseCase interface {
	Execute(ctx context.Context, in input.ConsumeMagicTokenInput) (usecases.ConsumeResult, error)
}

type TryFallbackActivationUseCase interface {
	Execute(ctx context.Context, fromE164 string) (usecases.FallbackResult, error)
}

type StartBudgetConfigurationUseCase interface {
	Execute(ctx context.Context, in usecases.StartBudgetConfigurationInput) (usecases.StartBudgetConfigurationResult, error)
}

type WhatsAppGateway interface {
	SendTextMessage(ctx context.Context, toE164, text string) error
}

type WhatsAppMessageProcessor struct {
	consumeUseCase     ConsumeMagicTokenUseCase
	fallbackUseCase    TryFallbackActivationUseCase
	startBudgetUseCase StartBudgetConfigurationUseCase
	waGateway          WhatsAppGateway
	messages           map[string]string
	o11y               observability.Observability
	inboundMessages    observability.Counter
	confirmationFailed observability.Counter
}

func NewWhatsAppMessageProcessor(
	consumeUseCase ConsumeMagicTokenUseCase,
	fallbackUseCase TryFallbackActivationUseCase,
	startBudgetUseCase StartBudgetConfigurationUseCase,
	waGateway WhatsAppGateway,
	messages map[string]string,
	o11y observability.Observability,
) *WhatsAppMessageProcessor {
	return &WhatsAppMessageProcessor{
		consumeUseCase:     consumeUseCase,
		fallbackUseCase:    fallbackUseCase,
		startBudgetUseCase: startBudgetUseCase,
		waGateway:          waGateway,
		messages:           messages,
		o11y:               o11y,
		inboundMessages: o11y.Metrics().Counter(
			"meta_inbound_messages_total",
			"Total de mensagens inbound recebidas do WhatsApp",
			"1",
		),
		confirmationFailed: o11y.Metrics().Counter(
			"onboarding_confirmation_failed_total",
			"Total de falhas ao enviar confirmacao de ativacao ao cliente",
			"1",
		),
	}
}

func (p *WhatsAppMessageProcessor) HandleActivation(ctx context.Context, fromE164, token string) error {
	p.inboundMessages.Add(ctx, 1, observability.String("kind", "activation_cmd"))

	from, err := identityvo.NewWhatsAppNumber(fromE164)
	if err != nil {
		slog.InfoContext(ctx, "onboarding.processor.invalid_country",
			"from", payload.MaskMobile(fromE164),
		)
		p.sendMessage(ctx, fromE164, p.msg("invalid_country"))
		return nil
	}

	result, execErr := p.consumeUseCase.Execute(ctx, input.ConsumeMagicTokenInput{
		Token:          token,
		FromE164:       from.String(),
		ActivationPath: valueobjects.ActivationPathDirect,
	})
	if execErr != nil {
		slog.ErrorContext(ctx, "onboarding.processor.consume_failed",
			"from", payload.MaskMobile(fromE164),
			"error", execErr.Error(),
		)
		p.sendMessage(ctx, from.String(), p.msg("system_unavailable_retry"))
		return fmt.Errorf("onboarding.processor: consume token: %w", execErr)
	}

	replyKey := consumeOutcomeToMessageKey(result.Outcome)
	p.sendMessage(ctx, from.String(), p.msg(replyKey))

	if result.Outcome == usecases.ConsumeOutcomeActivated {
		p.startOnboarding(ctx, from.String(), fromE164, result.UserID)
	}
	return nil
}

func (p *WhatsAppMessageProcessor) startOnboarding(ctx context.Context, _, fromE164, userID string) {
	parsedUserID, parseErr := uuid.Parse(userID)
	if parseErr != nil {
		slog.WarnContext(ctx, "onboarding.processor.start_budget_invalid_user",
			"from", payload.MaskMobile(fromE164),
			"error", parseErr.Error(),
		)
		return
	}

	if _, startErr := p.startBudgetUseCase.Execute(ctx, usecases.StartBudgetConfigurationInput{
		UserID:  parsedUserID,
		Channel: entities.OnboardingChannelWhatsApp,
	}); startErr != nil {
		slog.WarnContext(ctx, "onboarding.processor.start_budget_failed",
			"from", payload.MaskMobile(fromE164),
			"error", startErr.Error(),
		)
	}
}

func (p *WhatsAppMessageProcessor) HandleFallback(ctx context.Context, fromE164 string) error {
	p.inboundMessages.Add(ctx, 1, observability.String("kind", "fallback_candidate"))

	from, err := identityvo.NewWhatsAppNumber(fromE164)
	if err != nil {
		slog.InfoContext(ctx, "onboarding.processor.invalid_country_fallback",
			"from", payload.MaskMobile(fromE164),
		)
		p.sendMessage(ctx, fromE164, p.msg("invalid_country"))
		return nil
	}

	result, execErr := p.fallbackUseCase.Execute(ctx, from.String())
	if execErr != nil {
		slog.WarnContext(ctx, "onboarding.processor.fallback_failed",
			"from", payload.MaskMobile(fromE164),
			"error", execErr.Error(),
		)
		return fmt.Errorf("onboarding.processor: fallback activation: %w", execErr)
	}

	switch result.Outcome {
	case usecases.FallbackOutcomeActivated:
		p.sendMessage(ctx, from.String(), p.msg("welcome_activated"))
	case usecases.FallbackOutcomeOutreachRequired:
		p.sendMessage(ctx, from.String(), p.msg("please_use_ativar_command"))
	}
	return nil
}

func (p *WhatsAppMessageProcessor) sendMessage(ctx context.Context, toE164, text string) {
	if err := p.waGateway.SendTextMessage(ctx, toE164, text); err != nil {
		p.confirmationFailed.Add(ctx, 1, observability.String("reason", "send_error"))
		slog.WarnContext(ctx, "onboarding.processor.send_failed",
			"to", payload.MaskMobile(toE164),
			"error", err.Error(),
		)
	}
}

func (p *WhatsAppMessageProcessor) msg(key string) string {
	if v, ok := p.messages[key]; ok {
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
	case usecases.ConsumeOutcomeUnsupportedCountry:
		return "invalid_country"
	default:
		return "system_unavailable_retry"
	}
}
