package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/input"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type ConsumeInternalResult struct {
	magicToken entities.MagicToken
	signal     *entities.SupportSignal
}

type ConsumeMagicToken struct {
	uow                uow.UnitOfWork[ConsumeInternalResult]
	factory            appinterfaces.RepositoryFactory
	identityGateway    appinterfaces.IdentityGateway
	subscriptionBinder appinterfaces.SubscriptionBinder
	publisher          outbox.Publisher
	idGen              id.Generator
	o11y               observability.Observability
	tokensConsumed     observability.Counter
	reuseAttempts      observability.Counter
	consumeLatency     observability.Histogram
	paidToConsumed     observability.Histogram
}

func NewConsumeMagicToken(
	u uow.UnitOfWork[ConsumeInternalResult],
	factory appinterfaces.RepositoryFactory,
	identityGateway appinterfaces.IdentityGateway,
	subscriptionBinder appinterfaces.SubscriptionBinder,
	publisher outbox.Publisher,
	idGen id.Generator,
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
		uow:                u,
		factory:            factory,
		identityGateway:    identityGateway,
		subscriptionBinder: subscriptionBinder,
		publisher:          publisher,
		idGen:              idGen,
		o11y:               o11y,
		tokensConsumed:     tokensConsumed,
		reuseAttempts:      reuseAttempts,
		consumeLatency:     consumeLatency,
		paidToConsumed:     paidToConsumed,
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
)

type ConsumeResult struct {
	Outcome ConsumeOutcome
}

func (uc *ConsumeMagicToken) Execute(ctx context.Context, in input.ConsumeMagicTokenInput) (ConsumeResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "onboarding.usecase.consume_magic_token")
	defer span.End()

	start := time.Now()

	token, err := valueobjects.TokenFromClear(in.Token)
	if err != nil {
		return ConsumeResult{Outcome: ConsumeOutcomeNotFound}, nil
	}

	result, err := uc.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (ConsumeInternalResult, error) {
		return uc.executeInTx(ctx, tx, token, in)
	})

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
	userResult, upsertErr := uc.identityGateway.UpsertUserByWhatsApp(ctx, in.FromE164, magicToken.CustomerEmail())
	if upsertErr != nil {
		return ConsumeInternalResult{}, fmt.Errorf("onboarding: consume magic token: upsert user: %w", upsertErr)
	}
	if bindErr := uc.subscriptionBinder.BindUser(ctx, magicToken.SubscriptionID(), userResult.UserID); bindErr != nil {
		return ConsumeInternalResult{}, fmt.Errorf("onboarding: consume magic token: bind subscription: %w", bindErr)
	}

	consumed, markErr := magicToken.MarkConsumed(userResult.UserID, in.FromE164, in.ActivationPath, now)
	if markErr != nil {
		return ConsumeInternalResult{}, fmt.Errorf("onboarding: consume magic token: mark consumed: %w", markErr)
	}

	if updateErr := tokenRepo.UpdateMarkConsumed(ctx, consumed); updateErr != nil {
		return ConsumeInternalResult{}, fmt.Errorf("onboarding: consume magic token: update consumed: %w", updateErr)
	}

	payload, payloadErr := buildSubscriptionBoundPayload(uc.idGen.NewID(), userResult.UserID, consumed, in.ActivationPath, now)
	if payloadErr != nil {
		return ConsumeInternalResult{}, fmt.Errorf("onboarding: consume magic token: %w", payloadErr)
	}

	evt, evtErr := outbox.NewEvent(outbox.EventInput{
		Type:          "onboarding.subscription_bound",
		AggregateType: "onboarding_token",
		AggregateID:   consumed.ID(),
		Payload:       payload,
		OccurredAt:    now,
	})
	if evtErr != nil {
		return ConsumeInternalResult{}, fmt.Errorf("onboarding: consume magic token: build event: %w", evtErr)
	}

	if pubErr := uc.publisher.Publish(ctx, evt); pubErr != nil {
		return ConsumeInternalResult{}, fmt.Errorf("onboarding: consume magic token: publish event: %w", pubErr)
	}

	slog.InfoContext(ctx, "onboarding.token.consumed",
		"token_hash_prefix", token.HashPrefix(),
		"user_id", userResult.UserID,
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
		return ConsumeResult{Outcome: ConsumeOutcomeActivated}, nil
	}

	switch {
	case errors.Is(err, domain.ErrTokenNotFound):
		return ConsumeResult{Outcome: ConsumeOutcomeNotFound}, nil
	case errors.Is(err, domain.ErrTokenExpired):
		return ConsumeResult{Outcome: ConsumeOutcomeExpired}, nil
	case errors.Is(err, domain.ErrTokenNotYetPaid):
		return ConsumeResult{Outcome: ConsumeOutcomeNotYetPaid}, nil
	case errors.Is(err, domain.ErrTokenAlreadyConsumedSame):
		return ConsumeResult{Outcome: ConsumeOutcomeAlreadyActive}, nil
	case errors.Is(err, domain.ErrTokenAlreadyConsumedOther):
		uc.reuseAttempts.Add(ctx, 1, observability.String("reason", "different_number"))
		slog.WarnContext(ctx, "onboarding.token.reuse_attempt",
			"token_hash_prefix", token.HashPrefix(),
			"from_mobile_masked", maskMobile(in.FromE164),
		)
		return ConsumeResult{Outcome: ConsumeOutcomeReuseOtherAccount}, nil
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
	case errors.Is(err, domain.ErrTokenNotYetPaid):
		return "not_yet_paid"
	case errors.Is(err, domain.ErrTokenAlreadyConsumedSame):
		return "already_active"
	case errors.Is(err, domain.ErrTokenAlreadyConsumedOther):
		return "reuse_other"
	default:
		return "error"
	}
}

func buildSubscriptionBoundPayload(eventID, userID string, token entities.MagicToken, path valueobjects.ActivationPath, boundAt time.Time) ([]byte, error) {
	prefix := ""
	if len(token.TokenHash()) > 0 {
		h := fmt.Sprintf("%x", token.TokenHash())
		if len(h) > 8 {
			prefix = h[:8]
		} else {
			prefix = h
		}
	}
	b, err := json.Marshal(map[string]any{
		"event_id":          eventID,
		"user_id":           userID,
		"subscription_id":   token.SubscriptionID(),
		"token_hash_prefix": prefix,
		"activation_path":   path.String(),
		"bound_at":          boundAt,
	})
	if err != nil {
		return nil, fmt.Errorf("onboarding: build subscription bound payload: %w", err)
	}
	return b, nil
}

func maskMobile(mobile string) string {
	if len(mobile) < 4 {
		return "****"
	}
	return mobile[:3] + "****" + mobile[len(mobile)-4:]
}
