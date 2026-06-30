package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/binding"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/input"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
)

type ConsumeInternalResult struct {
	magicToken entities.MagicToken
	signal     *entities.SupportSignal
}

type ConsumeMagicToken struct {
	uow              uow.UnitOfWork
	factory          appinterfaces.RepositoryFactory
	binding          *binding.SubscriptionBindingService
	idGen            id.Generator
	activationWindow time.Duration
	o11y             observability.Observability
	tokensConsumed   observability.Counter
	reuseAttempts    observability.Counter
	consumeLatency   observability.Histogram
	paidToConsumed   observability.Histogram
}

func NewConsumeMagicToken(
	u uow.UnitOfWork,
	factory appinterfaces.RepositoryFactory,
	binding *binding.SubscriptionBindingService,
	idGen id.Generator,
	activationWindow time.Duration,
	o11y observability.Observability,
) *ConsumeMagicToken {
	tokensConsumed := o11y.Metrics().Counter(
		"onboarding_tokens_consumed_total",
		"Total de tokens consumidos por caminho de ativacao",
		"1",
	)
	reuseAttempts := o11y.Metrics().Counter(
		"onboarding_token_reuse_attempt_total",
		"Total de tentativas de reuso de token por numero diferente",
		"1",
	)
	consumeLatency := o11y.Metrics().Histogram(
		"onboarding_consume_latency_seconds",
		"Latencia do use case ConsumeMagicToken",
		"s",
	)
	paidToConsumed := o11y.Metrics().Histogram(
		"onboarding_paid_to_consumed_seconds",
		"Tempo entre token pago e consumido por caminho de ativacao",
		"s",
	)
	return &ConsumeMagicToken{
		uow:              u,
		factory:          factory,
		binding:          binding,
		idGen:            idGen,
		activationWindow: activationWindow,
		o11y:             o11y,
		tokensConsumed:   tokensConsumed,
		reuseAttempts:    reuseAttempts,
		consumeLatency:   consumeLatency,
		paidToConsumed:   paidToConsumed,
	}
}

type ConsumeOutcome uint8

const (
	ConsumeOutcomeActivated ConsumeOutcome = iota + 1
	ConsumeOutcomeAlreadyActive
	ConsumeOutcomeReuseOtherAccount
	ConsumeOutcomeNotYetPaid
	ConsumeOutcomeExpired
	ConsumeOutcomeNotFound
	ConsumeOutcomeUnsupportedCountry
)

type ConsumeResult struct {
	Outcome ConsumeOutcome
	UserID  string
}

func (uc *ConsumeMagicToken) Execute(ctx context.Context, in input.ConsumeMagicTokenInput) (ConsumeResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "onboarding.usecase.consume_magic_token")
	defer span.End()

	if err := in.Validate(); err != nil {
		span.RecordError(err)
		return ConsumeResult{}, err
	}

	start := time.Now()

	token, err := valueobjects.TokenFromClear(in.Token)
	if err != nil {
		return ConsumeResult{Outcome: ConsumeOutcomeNotFound}, nil
	}

	var result ConsumeInternalResult
	err = uc.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) error {
		res, txErr := uc.executeInTx(ctx, tx, token, in)
		result = res
		return txErr
	})
	if err != nil && errors.Is(err, domain.ErrTokenAlreadyConsumedOther) && result.signal != nil && uc.uow.DBTX() != nil {
		signalRepo := uc.factory.SupportSignalRepository(uc.uow.DBTX())
		if insertErr := signalRepo.Insert(ctx, *result.signal); insertErr != nil {
			span.RecordError(insertErr)
			return ConsumeResult{}, fmt.Errorf("onboarding: consume magic token: persist reuse signal: %w", insertErr)
		}
	}

	elapsed := time.Since(start).Seconds()
	uc.consumeLatency.Record(ctx, elapsed, observability.String("result", consumeResultLabel(err)))

	return uc.mapResult(ctx, token, in, result, err)
}

func (uc *ConsumeMagicToken) executeInTx(ctx context.Context, tx database.DBTX, token valueobjects.Token, in input.ConsumeMagicTokenInput) (ConsumeInternalResult, error) {
	tokenRepo := uc.factory.MagicTokenRepository(tx)
	signalRepo := uc.factory.SupportSignalRepository(tx)

	magicToken, findErr := tokenRepo.FindByHash(ctx, token.Hash())
	if findErr != nil {
		if errors.Is(findErr, domain.ErrTokenNotFound) {
			return ConsumeInternalResult{}, domain.ErrTokenNotFound
		}
		return ConsumeInternalResult{}, fmt.Errorf("onboarding: consume magic token: find: %w", findErr)
	}

	now := time.Now().UTC()
	if magicToken.IsExpiredAt(now) && magicToken.Status() != valueobjects.TokenStatusConsumed {
		return ConsumeInternalResult{magicToken: magicToken}, domain.ErrTokenExpired
	}

	switch magicToken.Status() {
	case valueobjects.TokenStatusPending:
		return ConsumeInternalResult{magicToken: magicToken}, domain.ErrTokenNotYetPaid
	case valueobjects.TokenStatusExpired:
		return ConsumeInternalResult{magicToken: magicToken}, domain.ErrTokenExpired
	case valueobjects.TokenStatusConsumed:
		return uc.handleConsumedToken(ctx, token, in.FromE164, magicToken, signalRepo, now)
	case valueobjects.TokenStatusPaid:
		return uc.handlePaidToken(ctx, token, in, magicToken, tokenRepo, now)
	}

	return ConsumeInternalResult{}, fmt.Errorf("onboarding: consume magic token: unexpected status: %s", magicToken.Status())
}

func (uc *ConsumeMagicToken) handleConsumedToken(ctx context.Context, token valueobjects.Token, fromE164 string, magicToken entities.MagicToken, signalRepo appinterfaces.SupportSignalRepository, now time.Time) (ConsumeInternalResult, error) {
	if magicToken.ConsumedByMobileE164() == fromE164 {
		return ConsumeInternalResult{magicToken: magicToken}, domain.ErrTokenAlreadyConsumedSame
	}

	payloadBytes, jsonErr := json.Marshal(map[string]any{
		"token_hash_prefix":  token.HashPrefix(),
		"from_mobile_masked": maskMobile(fromE164),
		"consumed_by_masked": maskMobile(magicToken.ConsumedByMobileE164()),
		"occurred_at":        now,
	})
	if jsonErr != nil {
		return ConsumeInternalResult{magicToken: magicToken}, fmt.Errorf("onboarding: consume magic token: marshal signal: %w", jsonErr)
	}

	sig, sigErr := entities.NewSupportSignal(uc.idGen.NewID(), valueobjects.SupportSignalKindTokenReuseAttempt, payloadBytes)
	if sigErr != nil {
		return ConsumeInternalResult{magicToken: magicToken}, fmt.Errorf("onboarding: consume magic token: new signal: %w", sigErr)
	}
	if insertErr := signalRepo.Insert(ctx, sig); insertErr != nil {
		return ConsumeInternalResult{magicToken: magicToken}, fmt.Errorf("onboarding: consume magic token: insert signal: %w", insertErr)
	}
	return ConsumeInternalResult{magicToken: magicToken, signal: &sig}, domain.ErrTokenAlreadyConsumedOther
}

func (uc *ConsumeMagicToken) handlePaidToken(ctx context.Context, token valueobjects.Token, in input.ConsumeMagicTokenInput, magicToken entities.MagicToken, tokenRepo appinterfaces.MagicTokenRepository, now time.Time) (ConsumeInternalResult, error) {
	if uc.activationWindow > 0 && !magicToken.IsActivationWindowOpen(now, uc.activationWindow) {
		return ConsumeInternalResult{magicToken: magicToken}, domain.ErrActivationWindowClosed
	}
	consumed, err := uc.binding.BindAndConsume(ctx, tokenRepo, magicToken, in.FromE164, in.ActivationPath, now)
	if err != nil {
		slog.ErrorContext(ctx, "onboarding.token.bind_consume_failed",
			"token_hash_prefix", token.HashPrefix(),
			"activation_path", in.ActivationPath.String(),
			"error", err.Error(),
		)
		return ConsumeInternalResult{}, fmt.Errorf("onboarding: consume magic token: %w", err)
	}

	slog.InfoContext(ctx, "onboarding.token.consumed",
		"token_hash_prefix", token.HashPrefix(),
		"user_id", consumed.ConsumedByUserID(),
		"activation_path", in.ActivationPath.String(),
	)
	return ConsumeInternalResult{magicToken: consumed}, nil
}

func (uc *ConsumeMagicToken) mapResult(ctx context.Context, token valueobjects.Token, in input.ConsumeMagicTokenInput, result ConsumeInternalResult, err error) (ConsumeResult, error) {
	if err == nil {
		uc.tokensConsumed.Add(ctx, 1, observability.String("activation_path", in.ActivationPath.String()))
		if !result.magicToken.PaidAt().IsZero() {
			uc.paidToConsumed.Record(ctx, time.Since(result.magicToken.PaidAt()).Seconds(),
				observability.String("activation_path", in.ActivationPath.String()))
		}
		return ConsumeResult{Outcome: ConsumeOutcomeActivated, UserID: result.magicToken.ConsumedByUserID()}, nil
	}

	switch {
	case errors.Is(err, domain.ErrTokenNotFound):
		return ConsumeResult{Outcome: ConsumeOutcomeNotFound}, nil
	case errors.Is(err, domain.ErrTokenExpired):
		return ConsumeResult{Outcome: ConsumeOutcomeExpired}, nil
	case errors.Is(err, domain.ErrActivationWindowClosed):
		return ConsumeResult{Outcome: ConsumeOutcomeExpired}, nil
	case errors.Is(err, domain.ErrTokenNotYetPaid):
		return ConsumeResult{Outcome: ConsumeOutcomeNotYetPaid}, nil
	case errors.Is(err, domain.ErrTokenAlreadyConsumedSame):
		return ConsumeResult{Outcome: ConsumeOutcomeAlreadyActive}, nil
	case errors.Is(err, domain.ErrTokenAlreadyConsumedRace):
		return ConsumeResult{Outcome: ConsumeOutcomeAlreadyActive}, nil
	case errors.Is(err, domain.ErrTokenAlreadyConsumedOther):
		uc.reuseAttempts.Add(ctx, 1, observability.String("reason", "different_number"))
		slog.WarnContext(ctx, "onboarding.token.reuse_attempt",
			"token_hash_prefix", token.HashPrefix(),
			"from_mobile_masked", maskMobile(in.FromE164),
		)
		return ConsumeResult{Outcome: ConsumeOutcomeReuseOtherAccount}, nil
	case errors.Is(err, domain.ErrUnsupportedCountry):
		return ConsumeResult{Outcome: ConsumeOutcomeUnsupportedCountry}, nil
	}
	return ConsumeResult{}, err
}

func consumeResultLabel(err error) string {
	switch {
	case err == nil:
		return "activated"
	case errors.Is(err, domain.ErrTokenNotFound):
		return "not_found"
	case errors.Is(err, domain.ErrTokenExpired):
		return "expired"
	case errors.Is(err, domain.ErrActivationWindowClosed):
		return "window_closed"
	case errors.Is(err, domain.ErrTokenNotYetPaid):
		return "not_yet_paid"
	case errors.Is(err, domain.ErrTokenAlreadyConsumedSame):
		return "already_active"
	case errors.Is(err, domain.ErrTokenAlreadyConsumedRace):
		return "already_active_race"
	case errors.Is(err, domain.ErrTokenAlreadyConsumedOther):
		return "reuse_other"
	case errors.Is(err, domain.ErrUnsupportedCountry):
		return "unsupported_country"
	default:
		return "error"
	}
}
