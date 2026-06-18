package usecases

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/binding"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
)

type TryFallbackActivation struct {
	uow            uow.UnitOfWork
	factory        appinterfaces.RepositoryFactory
	binding        *binding.SubscriptionBindingService
	o11y           observability.Observability
	tokensConsumed observability.Counter
}

func NewTryFallbackActivation(
	u uow.UnitOfWork,
	factory appinterfaces.RepositoryFactory,
	binding *binding.SubscriptionBindingService,
	o11y observability.Observability,
) *TryFallbackActivation {
	tokensConsumed := o11y.Metrics().Counter(
		"onboarding_tokens_consumed_total",
		"Total de tokens consumidos por caminho de ativacao",
		"1",
	)
	return &TryFallbackActivation{
		uow:            u,
		factory:        factory,
		binding:        binding,
		o11y:           o11y,
		tokensConsumed: tokensConsumed,
	}
}

type FallbackOutcome uint8

const (
	FallbackOutcomeActivated FallbackOutcome = iota + 1
	FallbackOutcomeNoMatch
	FallbackOutcomeOutreachRequired
)

type FallbackResult struct {
	Outcome FallbackOutcome
}

func (uc *TryFallbackActivation) Execute(ctx context.Context, fromE164 string) (FallbackResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "onboarding.usecase.try_fallback_activation")
	defer span.End()

	err := uc.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) error {
		_, txErr := uc.executeInTx(ctx, tx, fromE164)
		return txErr
	})

	if err != nil {
		return uc.mapError(ctx, fromE164, err)
	}

	uc.tokensConsumed.Add(ctx, 1, observability.String("activation_path", valueobjects.ActivationPathFallbackE164.String()))
	return FallbackResult{Outcome: FallbackOutcomeActivated}, nil
}

func (uc *TryFallbackActivation) executeInTx(ctx context.Context, tx database.DBTX, fromE164 string) (ConsumeInternalResult, error) {
	tokenRepo := uc.factory.MagicTokenRepository(tx)
	magicToken, err := uc.findToken(ctx, tokenRepo, fromE164)
	if err != nil {
		return ConsumeInternalResult{}, err
	}

	now := time.Now().UTC()
	if magicToken.IsExpiredAt(now) {
		return ConsumeInternalResult{magicToken: magicToken}, domain.ErrTokenExpired
	}

	consumed, err := uc.binding.BindAndConsume(ctx, tokenRepo, magicToken, fromE164, valueobjects.ActivationPathFallbackE164, now)
	if err != nil {
		return ConsumeInternalResult{}, fmt.Errorf("onboarding: try fallback activation: %w", err)
	}

	slog.InfoContext(ctx, "onboarding.token.consumed",
		"token_id", consumed.ID(),
		"activation_path", valueobjects.ActivationPathFallbackE164.String(),
	)
	return ConsumeInternalResult{magicToken: consumed}, nil
}

func (uc *TryFallbackActivation) findToken(ctx context.Context, tokenRepo appinterfaces.MagicTokenRepository, fromE164 string) (entities.MagicToken, error) {
	magicToken, err := tokenRepo.FindPaidByMobileForFallback(ctx, fromE164)
	if err != nil {
		if errors.Is(err, domain.ErrTokenNotFound) {
			return entities.MagicToken{}, domain.ErrTokenNotFound
		}
		return entities.MagicToken{}, fmt.Errorf("onboarding: try fallback activation: find: %w", err)
	}
	if !magicToken.HasOutreach() {
		return magicToken, domain.ErrTokenNotYetPaid
	}
	return magicToken, nil
}

func (uc *TryFallbackActivation) mapError(ctx context.Context, fromE164 string, err error) (FallbackResult, error) {
	switch {
	case errors.Is(err, domain.ErrTokenNotFound):
		slog.InfoContext(ctx, "onboarding.fallback.no_match", "from_mobile_masked", maskMobile(fromE164))
		return FallbackResult{Outcome: FallbackOutcomeNoMatch}, nil
	case errors.Is(err, domain.ErrTokenNotYetPaid):
		slog.InfoContext(ctx, "onboarding.fallback.outreach_required", "from_mobile_masked", maskMobile(fromE164))
		return FallbackResult{Outcome: FallbackOutcomeOutreachRequired}, nil
	case errors.Is(err, domain.ErrTokenExpired):
		slog.InfoContext(ctx, "onboarding.fallback.no_match", "from_mobile_masked", maskMobile(fromE164))
		return FallbackResult{Outcome: FallbackOutcomeNoMatch}, nil
	default:
		return FallbackResult{}, err
	}
}
