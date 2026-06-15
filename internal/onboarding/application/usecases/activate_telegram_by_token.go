package usecases

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	identityapp "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	identityinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	identityentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	identityvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

const channelTelegram = "telegram"

type ActivateTelegramOutcome uint8

const (
	ActivateTelegramOutcomeLinked ActivateTelegramOutcome = iota + 1
	ActivateTelegramOutcomeAlreadyLinked
	ActivateTelegramOutcomeRequiresWhatsAppActivation
	ActivateTelegramOutcomeNotYetPaid
	ActivateTelegramOutcomeExpired
	ActivateTelegramOutcomeNotFound
	ActivateTelegramOutcomeReusedOtherAccount
)

func (o ActivateTelegramOutcome) String() string {
	switch o {
	case ActivateTelegramOutcomeLinked:
		return "linked"
	case ActivateTelegramOutcomeAlreadyLinked:
		return "already_linked"
	case ActivateTelegramOutcomeRequiresWhatsAppActivation:
		return "requires_whatsapp_activation"
	case ActivateTelegramOutcomeNotYetPaid:
		return "not_yet_paid"
	case ActivateTelegramOutcomeExpired:
		return "expired"
	case ActivateTelegramOutcomeNotFound:
		return "not_found"
	case ActivateTelegramOutcomeReusedOtherAccount:
		return "reused_other_account"
	default:
		return "invalid"
	}
}

type ActivateTelegramResult struct {
	Outcome ActivateTelegramOutcome
	UserID  uuid.UUID
}

type ActivateTelegramByTokenInput struct {
	Token          string
	TelegramUserID int64
}

type ActivateTelegramByToken struct {
	factory         interfaces.RepositoryFactory
	identityFactory identityinterfaces.RepositoryFactory
	uow             uow.UnitOfWork[ActivateTelegramResult]
	o11y            observability.Observability
	outcomesTotal   observability.Counter
}

func NewActivateTelegramByToken(
	factory interfaces.RepositoryFactory,
	identityFactory identityinterfaces.RepositoryFactory,
	u uow.UnitOfWork[ActivateTelegramResult],
	o11y observability.Observability,
) *ActivateTelegramByToken {
	outcomes := o11y.Metrics().Counter(
		"onboarding_activate_telegram_outcome_total",
		"Total de outcomes da ativacao do Telegram por tipo",
		"1",
	)
	return &ActivateTelegramByToken{
		factory:         factory,
		identityFactory: identityFactory,
		uow:             u,
		o11y:            o11y,
		outcomesTotal:   outcomes,
	}
}

func (uc *ActivateTelegramByToken) Execute(ctx context.Context, in ActivateTelegramByTokenInput) (ActivateTelegramResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "onboarding.usecase.activate_telegram_by_token")
	defer span.End()

	if in.TelegramUserID <= 0 {
		return uc.observe(ctx, ActivateTelegramResult{Outcome: ActivateTelegramOutcomeNotFound}), nil
	}

	token, err := valueobjects.TokenFromClear(in.Token)
	if err != nil {
		return uc.observe(ctx, ActivateTelegramResult{Outcome: ActivateTelegramOutcomeNotFound}), nil
	}

	res, err := uc.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (ActivateTelegramResult, error) {
		return uc.executeInTx(ctx, tx, token, in.TelegramUserID)
	})
	if err != nil {
		span.RecordError(err)
		return ActivateTelegramResult{}, fmt.Errorf("onboarding.activate_telegram_by_token: %w", err)
	}
	return uc.observe(ctx, res), nil
}

func (uc *ActivateTelegramByToken) executeInTx(
	ctx context.Context,
	tx database.DBTX,
	token valueobjects.Token,
	telegramUserID int64,
) (ActivateTelegramResult, error) {
	tokenRepo := uc.factory.MagicTokenRepository(tx)
	identityRepo := uc.identityFactory.UserIdentityRepository(tx)

	magicToken, findErr := tokenRepo.FindByHash(ctx, token.Hash())
	if findErr != nil {
		if errors.Is(findErr, domain.ErrTokenNotFound) {
			return ActivateTelegramResult{Outcome: ActivateTelegramOutcomeNotFound}, nil
		}
		return ActivateTelegramResult{}, fmt.Errorf("find token: %w", findErr)
	}

	now := time.Now().UTC()
	if magicToken.IsExpiredAt(now) && magicToken.Status() != valueobjects.TokenStatusConsumed {
		return ActivateTelegramResult{Outcome: ActivateTelegramOutcomeExpired}, nil
	}

	switch magicToken.Status() {
	case valueobjects.TokenStatusPending:
		return ActivateTelegramResult{Outcome: ActivateTelegramOutcomeNotYetPaid}, nil
	case valueobjects.TokenStatusExpired:
		return ActivateTelegramResult{Outcome: ActivateTelegramOutcomeExpired}, nil
	case valueobjects.TokenStatusPaid:
		return ActivateTelegramResult{Outcome: ActivateTelegramOutcomeRequiresWhatsAppActivation}, nil
	case valueobjects.TokenStatusConsumed:
		return uc.linkInTx(ctx, identityRepo, magicToken.ConsumedByUserID(), telegramUserID, now)
	default:
		return ActivateTelegramResult{}, fmt.Errorf("unexpected status: %s", magicToken.Status())
	}
}

func (uc *ActivateTelegramByToken) linkInTx(
	ctx context.Context,
	repo identityinterfaces.UserIdentityRepository,
	userIDRaw string,
	telegramUserID int64,
	now time.Time,
) (ActivateTelegramResult, error) {
	if userIDRaw == "" {
		return ActivateTelegramResult{Outcome: ActivateTelegramOutcomeRequiresWhatsAppActivation}, nil
	}
	userID, parseErr := uuid.Parse(userIDRaw)
	if parseErr != nil {
		return ActivateTelegramResult{Outcome: ActivateTelegramOutcomeRequiresWhatsAppActivation}, nil
	}

	channel, err := identityvo.NewChannel(channelTelegram)
	if err != nil {
		return ActivateTelegramResult{}, fmt.Errorf("channel: %w", err)
	}
	externalID, err := identityvo.NewExternalID(channel, strconv.FormatInt(telegramUserID, 10))
	if err != nil {
		return ActivateTelegramResult{Outcome: ActivateTelegramOutcomeNotFound}, nil
	}

	existing, found, lookupErr := repo.TryFindActive(ctx, channel, externalID)
	if lookupErr != nil {
		return ActivateTelegramResult{}, fmt.Errorf("lookup existing: %w", lookupErr)
	}
	if found {
		if existing.UserID() == userID {
			return ActivateTelegramResult{Outcome: ActivateTelegramOutcomeAlreadyLinked, UserID: userID}, nil
		}
		return ActivateTelegramResult{Outcome: ActivateTelegramOutcomeReusedOtherAccount, UserID: userID}, nil
	}

	identityID, err := uuid.NewV7()
	if err != nil {
		return ActivateTelegramResult{}, fmt.Errorf("generate identity id: %w", err)
	}
	identity, err := identityentities.NewUserIdentity(identityID, userID, channel, externalID, now)
	if err != nil {
		return ActivateTelegramResult{}, fmt.Errorf("build identity: %w", err)
	}

	if insertErr := repo.Insert(ctx, identity); insertErr != nil {
		if errors.Is(insertErr, identityapp.ErrUserIdentityAlreadyLinked) {
			winner, winnerFound, reReadErr := repo.TryFindActive(ctx, channel, externalID)
			if reReadErr != nil {
				return ActivateTelegramResult{}, fmt.Errorf("post-conflict re-read: %w", reReadErr)
			}
			if winnerFound && winner.UserID() == userID {
				return ActivateTelegramResult{Outcome: ActivateTelegramOutcomeAlreadyLinked, UserID: userID}, nil
			}
			return ActivateTelegramResult{Outcome: ActivateTelegramOutcomeReusedOtherAccount, UserID: userID}, nil
		}
		return ActivateTelegramResult{}, fmt.Errorf("insert identity: %w", insertErr)
	}
	return ActivateTelegramResult{Outcome: ActivateTelegramOutcomeLinked, UserID: userID}, nil
}

func (uc *ActivateTelegramByToken) observe(ctx context.Context, res ActivateTelegramResult) ActivateTelegramResult {
	uc.outcomesTotal.Add(ctx, 1, observability.String("outcome", res.Outcome.String()))
	return res
}
