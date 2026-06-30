package usecases

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/binding"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/input"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
)

type tokenActivator interface {
	Execute(ctx context.Context, in input.ConsumeMagicTokenInput) (ConsumeResult, error)
}

type ActivateOutcome uint8

const (
	ActivateOutcomePhoneMatched ActivateOutcome = iota + 1
	ActivateOutcomeTokenMatched
	ActivateOutcomeAlreadyActive
	ActivateOutcomeNoMatch
)

func (o ActivateOutcome) String() string {
	switch o {
	case ActivateOutcomePhoneMatched:
		return "phone_matched"
	case ActivateOutcomeTokenMatched:
		return "token_matched"
	case ActivateOutcomeAlreadyActive:
		return "already_active"
	case ActivateOutcomeNoMatch:
		return "no_match"
	default:
		return "unknown"
	}
}

type ActivateFromInboundResult struct {
	Outcome ActivateOutcome
	UserID  string
}

type ActivateFromInbound struct {
	uow              uow.UnitOfWork
	factory          appinterfaces.RepositoryFactory
	binding          *binding.SubscriptionBindingService
	consumeToken     tokenActivator
	gateway          appinterfaces.WhatsAppGateway
	throttle         appinterfaces.NoMatchThrottle
	activationWindow time.Duration
	noMatchMsg       string
	o11y             observability.Observability
	attemptCounter   observability.Counter
	windowExpired    observability.Counter
}

func NewActivateFromInbound(
	u uow.UnitOfWork,
	factory appinterfaces.RepositoryFactory,
	b *binding.SubscriptionBindingService,
	consumeToken tokenActivator,
	gateway appinterfaces.WhatsAppGateway,
	throttle appinterfaces.NoMatchThrottle,
	activationWindow time.Duration,
	noMatchMsg string,
	o11y observability.Observability,
) *ActivateFromInbound {
	attemptCounter := o11y.Metrics().Counter(
		"onboarding_activation_attempt_total",
		"Total de tentativas de ativacao por outcome",
		"1",
	)
	windowExpired := o11y.Metrics().Counter(
		"onboarding_activation_window_expired_total",
		"Total de tentativas de ativacao fora da janela de ativacao",
		"1",
	)
	return &ActivateFromInbound{
		uow:              u,
		factory:          factory,
		binding:          b,
		consumeToken:     consumeToken,
		gateway:          gateway,
		throttle:         throttle,
		activationWindow: activationWindow,
		noMatchMsg:       noMatchMsg,
		o11y:             o11y,
		attemptCounter:   attemptCounter,
		windowExpired:    windowExpired,
	}
}

func (uc *ActivateFromInbound) Execute(ctx context.Context, in input.ActivateFromInboundInput) (ActivateFromInboundResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "onboarding.usecase.activate_from_inbound")
	defer span.End()

	if err := in.Validate(); err != nil {
		span.RecordError(err)
		return ActivateFromInboundResult{}, err
	}

	now := time.Now().UTC()

	result, err := uc.loadByPhone(ctx, in.PeerE164, now)
	if err != nil {
		span.RecordError(err)
		return ActivateFromInboundResult{}, err
	}
	if result.Outcome != 0 {
		uc.attemptCounter.Add(ctx, 1, observability.String("outcome", result.Outcome.String()))
		return result, nil
	}

	result, err = uc.tryToken(ctx, in)
	if err != nil {
		span.RecordError(err)
		return ActivateFromInboundResult{}, err
	}
	if result.Outcome != 0 {
		uc.attemptCounter.Add(ctx, 1, observability.String("outcome", result.Outcome.String()))
		return result, nil
	}

	result, err = uc.noMatch(ctx, in.PeerE164, now)
	if err != nil {
		span.RecordError(err)
		return ActivateFromInboundResult{}, err
	}
	uc.attemptCounter.Add(ctx, 1, observability.String("outcome", result.Outcome.String()))
	return result, nil
}

func (uc *ActivateFromInbound) loadByPhone(ctx context.Context, peerE164 string, now time.Time) (ActivateFromInboundResult, error) {
	paidAfter := now.Add(-uc.activationWindow)

	var result ActivateFromInboundResult
	err := uc.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) error {
		tokenRepo := uc.factory.MagicTokenRepository(tx)

		token, findErr := tokenRepo.FindActivableByMobile(ctx, peerE164, paidAfter)
		if findErr != nil {
			if errors.Is(findErr, domain.ErrTokenNotFound) {
				return nil
			}
			return fmt.Errorf("onboarding: activate_from_inbound: find: %w", findErr)
		}

		if markErr := tokenRepo.UpdateMarkActivationStartedAt(ctx, token.ID(), now); markErr != nil {
			return fmt.Errorf("onboarding: activate_from_inbound: mark_activation_started: %w", markErr)
		}

		consumed, bindErr := uc.binding.BindAndConsume(ctx, tokenRepo, token, peerE164, valueobjects.ActivationPathFallbackE164, now)
		if bindErr != nil {
			if errors.Is(bindErr, domain.ErrTokenAlreadyConsumedRace) {
				result = ActivateFromInboundResult{Outcome: ActivateOutcomeAlreadyActive}
				return nil
			}
			return fmt.Errorf("onboarding: activate_from_inbound: bind: %w", bindErr)
		}

		slog.InfoContext(ctx, "onboarding.activation.phone_matched",
			"user_id", consumed.ConsumedByUserID(),
			"from_mobile_masked", maskMobile(peerE164),
		)
		result = ActivateFromInboundResult{Outcome: ActivateOutcomePhoneMatched, UserID: consumed.ConsumedByUserID()}
		return nil
	})

	if err != nil {
		return ActivateFromInboundResult{}, err
	}
	return result, nil
}

func (uc *ActivateFromInbound) tryToken(ctx context.Context, in input.ActivateFromInboundInput) (ActivateFromInboundResult, error) {
	raw := extractTokenText(in.Text)
	if _, err := valueobjects.TokenFromClear(raw); err != nil {
		return ActivateFromInboundResult{}, nil
	}

	consumeResult, err := uc.consumeToken.Execute(ctx, input.ConsumeMagicTokenInput{
		Token:          raw,
		FromE164:       in.PeerE164,
		ActivationPath: valueobjects.ActivationPathDirect,
	})
	if err != nil {
		return ActivateFromInboundResult{}, fmt.Errorf("onboarding: activate_from_inbound: consume token: %w", err)
	}

	switch consumeResult.Outcome {
	case ConsumeOutcomeActivated:
		return ActivateFromInboundResult{Outcome: ActivateOutcomeTokenMatched, UserID: consumeResult.UserID}, nil
	case ConsumeOutcomeAlreadyActive:
		return ActivateFromInboundResult{Outcome: ActivateOutcomeAlreadyActive, UserID: consumeResult.UserID}, nil
	default:
		uc.windowExpired.Add(ctx, 1)
		return ActivateFromInboundResult{}, nil
	}
}

func (uc *ActivateFromInbound) noMatch(ctx context.Context, peerE164 string, now time.Time) (ActivateFromInboundResult, error) {
	var alreadyActive bool
	if checkErr := uc.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) error {
		found, hasErr := uc.factory.MagicTokenRepository(tx).HasConsumedByMobile(ctx, peerE164)
		if hasErr != nil {
			return hasErr
		}
		alreadyActive = found
		return nil
	}); checkErr != nil {
		return ActivateFromInboundResult{}, fmt.Errorf("onboarding: activate_from_inbound: has_consumed: %w", checkErr)
	}
	if alreadyActive {
		slog.InfoContext(ctx, "onboarding.activation.already_active_replay",
			"from_mobile_masked", maskMobile(peerE164),
		)
		return ActivateFromInboundResult{Outcome: ActivateOutcomeAlreadyActive}, nil
	}

	windowStart := now.Truncate(uc.activationWindow)
	allowed, err := uc.throttle.AllowReply(ctx, peerE164, windowStart)
	if err != nil {
		return ActivateFromInboundResult{}, fmt.Errorf("onboarding: activate_from_inbound: throttle: %w", err)
	}

	slog.WarnContext(ctx, "onboarding.activation.no_match",
		"from_mobile_masked", maskMobile(peerE164),
		"throttle_allowed", allowed,
	)

	if allowed {
		if sendErr := uc.gateway.SendTextMessage(ctx, peerE164, uc.noMatchMsg); sendErr != nil {
			return ActivateFromInboundResult{}, fmt.Errorf("onboarding: activate_from_inbound: send no_match: %w", sendErr)
		}
	}

	return ActivateFromInboundResult{Outcome: ActivateOutcomeNoMatch}, nil
}

func extractTokenText(text string) string {
	trimmed := strings.TrimSpace(text)
	upper := strings.ToUpper(trimmed)
	if strings.HasPrefix(upper, "ATIVAR ") {
		return strings.TrimSpace(trimmed[len("ATIVAR "):])
	}
	return trimmed
}
