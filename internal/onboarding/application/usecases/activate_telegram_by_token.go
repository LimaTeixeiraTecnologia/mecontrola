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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/services"
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

type DirectActivationDecider interface {
	Decide(token entities.MagicToken, flagEnabled bool) services.DirectActivationDecision
}

type DirectActivationBinder interface {
	BindAndConsume(
		ctx context.Context,
		tokenRepo interfaces.MagicTokenRepository,
		magicToken entities.MagicToken,
		fromE164 string,
		path valueobjects.ActivationPath,
		now time.Time,
	) (entities.MagicToken, error)
}

type ActivateTelegramByToken struct {
	factory         interfaces.RepositoryFactory
	identityFactory identityinterfaces.RepositoryFactory
	uow             uow.UnitOfWork[ActivateTelegramResult]
	directDecider   DirectActivationDecider
	directBinder    DirectActivationBinder
	directEnabled   bool
	o11y            observability.Observability
	outcomesTotal   observability.Counter
	activationTotal observability.Counter
}

func NewActivateTelegramByToken(
	factory interfaces.RepositoryFactory,
	identityFactory identityinterfaces.RepositoryFactory,
	u uow.UnitOfWork[ActivateTelegramResult],
	directDecider DirectActivationDecider,
	directBinder DirectActivationBinder,
	directEnabled bool,
	o11y observability.Observability,
) *ActivateTelegramByToken {
	outcomes := o11y.Metrics().Counter(
		"onboarding_activate_telegram_outcome_total",
		"Total de outcomes da ativacao do Telegram por tipo",
		"1",
	)
	activation := o11y.Metrics().Counter(
		"onboarding_telegram_activation_total",
		"Total de ativacoes de Telegram por path de decisao",
		"1",
	)
	return &ActivateTelegramByToken{
		factory:         factory,
		identityFactory: identityFactory,
		uow:             u,
		directDecider:   directDecider,
		directBinder:    directBinder,
		directEnabled:   directEnabled,
		o11y:            o11y,
		outcomesTotal:   outcomes,
		activationTotal: activation,
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
		return uc.handlePaidInTx(ctx, tokenRepo, identityRepo, magicToken, telegramUserID, now)
	case valueobjects.TokenStatusConsumed:
		return uc.linkInTx(ctx, identityRepo, magicToken.ConsumedByUserID(), telegramUserID, now)
	default:
		return ActivateTelegramResult{}, fmt.Errorf("unexpected status: %s", magicToken.Status())
	}
}

func (uc *ActivateTelegramByToken) handlePaidInTx(
	ctx context.Context,
	tokenRepo interfaces.MagicTokenRepository,
	identityRepo identityinterfaces.UserIdentityRepository,
	magicToken entities.MagicToken,
	telegramUserID int64,
	now time.Time,
) (ActivateTelegramResult, error) {
	decision := uc.directDecider.Decide(magicToken, uc.directEnabled)
	switch decision.Outcome {
	case services.OutcomeRequiresWhatsAppActivation:
		uc.recordPath(ctx, "whatsapp_required")
		return ActivateTelegramResult{Outcome: ActivateTelegramOutcomeRequiresWhatsAppActivation}, nil
	case services.OutcomeDirectBlocked:
		uc.recordPath(ctx, "direct_blocked_missing_data")
		return ActivateTelegramResult{Outcome: ActivateTelegramOutcomeRequiresWhatsAppActivation}, nil
	case services.OutcomeDirectAllowed:
		telegramExternalID := strconv.FormatInt(telegramUserID, 10)
		if err := tokenRepo.UpdateTelegramExternalID(ctx, magicToken.ID(), telegramExternalID); err != nil {
			return ActivateTelegramResult{}, fmt.Errorf("persist telegram external id: %w", err)
		}
		linkedToken, err := magicToken.LinkTelegramExternalID(telegramExternalID)
		if err != nil {
			return ActivateTelegramResult{}, fmt.Errorf("link telegram external id: %w", err)
		}
		consumed, err := uc.directBinder.BindAndConsume(
			ctx,
			tokenRepo,
			linkedToken,
			decision.CustomerMobileE164,
			valueobjects.ActivationPathDirect,
			now,
		)
		if err != nil {
			return ActivateTelegramResult{}, fmt.Errorf("direct bind: %w", err)
		}
		uc.recordPath(ctx, "direct_linked")
		return uc.linkInTx(ctx, identityRepo, consumed.ConsumedByUserID(), telegramUserID, now)
	default:
		return ActivateTelegramResult{}, fmt.Errorf("unexpected direct decision: %s", decision.Outcome)
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

func (uc *ActivateTelegramByToken) recordPath(ctx context.Context, path string) {
	uc.activationTotal.Add(ctx, 1, observability.String("path", path))
}
