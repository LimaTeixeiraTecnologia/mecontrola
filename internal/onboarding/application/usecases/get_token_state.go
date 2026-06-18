package usecases

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/output"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type GetTokenState struct {
	repo             appinterfaces.MagicTokenRepository
	botNumber        string
	botNumberDisplay string
	telegramBot      string
	o11y             observability.Observability
}

func NewGetTokenState(
	repo appinterfaces.MagicTokenRepository,
	botNumber string,
	botNumberDisplay string,
	telegramBot string,
	o11y observability.Observability,
) *GetTokenState {
	return &GetTokenState{
		repo:             repo,
		botNumber:        botNumber,
		botNumberDisplay: botNumberDisplay,
		telegramBot:      telegramBot,
		o11y:             o11y,
	}
}

type TokenStateReason string

const (
	TokenStateReasonNotFound TokenStateReason = "not_found"
	TokenStateReasonPending  TokenStateReason = "pending"
	TokenStateReasonExpired  TokenStateReason = "expired"
	TokenStateReasonConsumed TokenStateReason = "consumed"
)

type GetTokenStateResult struct {
	Output output.GetTokenStateOutput
	Reason TokenStateReason
}

func (uc *GetTokenState) Execute(ctx context.Context, clearToken string) (GetTokenStateResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "onboarding.usecase.get_token_state")
	defer span.End()

	token, err := valueobjects.TokenFromClear(clearToken)
	if err != nil {
		return GetTokenStateResult{
			Output: output.GetTokenStateOutput{ReadyToActivate: false},
			Reason: TokenStateReasonNotFound,
		}, nil
	}

	magicToken, err := uc.repo.FindByHash(ctx, token.Hash())
	if err != nil {
		if errors.Is(err, domain.ErrTokenNotFound) {
			return GetTokenStateResult{
				Output: output.GetTokenStateOutput{ReadyToActivate: false},
				Reason: TokenStateReasonNotFound,
			}, nil
		}
		return GetTokenStateResult{}, fmt.Errorf("onboarding: get token state: find: %w", err)
	}

	now := time.Now().UTC()

	if magicToken.Status() == valueobjects.TokenStatusPaid && !magicToken.IsExpiredAt(now) {
		waMe := fmt.Sprintf("https://wa.me/%s?text=ATIVAR%%20%s", sanitizeE164(uc.botNumber), clearToken)
		var tgLink string
		if uc.telegramBot != "" {
			tgLink = fmt.Sprintf("https://t.me/%s?start=ATIVAR_%s", strings.TrimPrefix(uc.telegramBot, "@"), clearToken)
		}
		return GetTokenStateResult{
			Output: output.GetTokenStateOutput{
				ReadyToActivate:  true,
				WaMeURL:          waMe,
				TelegramDeepLink: tgLink,
				BotNumberDisplay: uc.botNumberDisplay,
			},
		}, nil
	}

	reason := reasonFromStatus(magicToken.Status(), magicToken.IsExpiredAt(now))
	return GetTokenStateResult{
		Output: output.GetTokenStateOutput{ReadyToActivate: false},
		Reason: reason,
	}, nil
}

func reasonFromStatus(status valueobjects.TokenStatus, expired bool) TokenStateReason {
	if expired {
		return TokenStateReasonExpired
	}
	switch status {
	case valueobjects.TokenStatusPending:
		return TokenStateReasonPending
	case valueobjects.TokenStatusExpired:
		return TokenStateReasonExpired
	case valueobjects.TokenStatusConsumed:
		return TokenStateReasonConsumed
	default:
		return TokenStateReasonNotFound
	}
}

func sanitizeE164(e164 string) string {
	if len(e164) > 0 && e164[0] == '+' {
		return e164[1:]
	}
	return e164
}
