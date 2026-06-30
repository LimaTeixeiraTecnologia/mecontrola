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
	o11y             observability.Observability
}

func NewGetTokenState(
	repo appinterfaces.MagicTokenRepository,
	botNumber string,
	botNumberDisplay string,
	o11y observability.Observability,
) *GetTokenState {
	return &GetTokenState{
		repo:             repo,
		botNumber:        botNumber,
		botNumberDisplay: botNumberDisplay,
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

	support := fmt.Sprintf("https://wa.me/%s", sanitizeE164(uc.botNumber))

	token, err := valueobjects.TokenFromClear(clearToken)
	if err != nil {
		return GetTokenStateResult{
			Output: output.GetTokenStateOutput{ReadyToActivate: false, Reason: string(TokenStateReasonNotFound), SupportURL: support},
			Reason: TokenStateReasonNotFound,
		}, nil
	}

	magicToken, err := uc.repo.FindByHash(ctx, token.Hash())
	if err != nil {
		if errors.Is(err, domain.ErrTokenNotFound) {
			return GetTokenStateResult{
				Output: output.GetTokenStateOutput{ReadyToActivate: false, Reason: string(TokenStateReasonNotFound), SupportURL: support},
				Reason: TokenStateReasonNotFound,
			}, nil
		}
		span.RecordError(err)
		return GetTokenStateResult{}, fmt.Errorf("onboarding: get token state: find: %w", err)
	}

	now := time.Now().UTC()

	if magicToken.Status() == valueobjects.TokenStatusPaid && !magicToken.IsExpiredAt(now) {
		waMe := buildWaMeURL(uc.botNumber, clearToken, magicToken.CustomerMobileE164())
		return GetTokenStateResult{
			Output: output.GetTokenStateOutput{
				ReadyToActivate:  true,
				WaMeURL:          waMe,
				BotNumberDisplay: uc.botNumberDisplay,
				SupportURL:       support,
			},
		}, nil
	}

	reason := reasonFromStatus(magicToken.Status(), magicToken.IsExpiredAt(now))
	out := output.GetTokenStateOutput{
		ReadyToActivate: false,
		Reason:          string(reason),
		SupportURL:      support,
	}
	if reason == TokenStateReasonConsumed {
		out.WaMeURL = buildWaMeURL(uc.botNumber, clearToken, magicToken.CustomerMobileE164())
		out.BotNumberDisplay = uc.botNumberDisplay
	}
	return GetTokenStateResult{Output: out, Reason: reason}, nil
}

func buildWaMeURL(botNumber, clearToken, customerMobileE164 string) string {
	bot := sanitizeE164(botNumber)
	if strings.TrimSpace(customerMobileE164) != "" {
		return fmt.Sprintf("https://wa.me/%s?text=Ativar+o+meu+plano", bot)
	}
	return fmt.Sprintf("https://wa.me/%s?text=%s", bot, clearToken)
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
