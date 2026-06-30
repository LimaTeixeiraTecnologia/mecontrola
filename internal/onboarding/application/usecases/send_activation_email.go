package usecases

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type ActivationTemplateInput struct {
	ActivationURL  string
	SupportURL     string
	ExpiresInHours int
}

type ActivationTemplate interface {
	Render(in ActivationTemplateInput) (html string, text string, err error)
}

type SendActivationEmail struct {
	repo              appinterfaces.MagicTokenRepository
	sender            appinterfaces.EmailSender
	template          ActivationTemplate
	boundChecker      appinterfaces.SubscriptionBoundChecker
	botNumber         string
	activationPageURL string
	fromAddress       string
	fromName          string
	replyTo           string
	subject           string
	tokenTTL          time.Duration
	o11y              observability.Observability
	dispatchedCtr     observability.Counter
}

func NewSendActivationEmail(
	repo appinterfaces.MagicTokenRepository,
	sender appinterfaces.EmailSender,
	template ActivationTemplate,
	botNumber string,
	activationPageURL string,
	fromAddress string,
	fromName string,
	replyTo string,
	tokenTTL time.Duration,
	boundChecker appinterfaces.SubscriptionBoundChecker,
	o11y observability.Observability,
) *SendActivationEmail {
	dispatchedCtr := o11y.Metrics().Counter(
		"onboarding_activation_email_dispatched_total",
		"Total de emails de ativacao enviados",
		"1",
	)
	return &SendActivationEmail{
		repo:              repo,
		sender:            sender,
		template:          template,
		boundChecker:      boundChecker,
		botNumber:         botNumber,
		activationPageURL: activationPageURL,
		fromAddress:       fromAddress,
		fromName:          fromName,
		replyTo:           replyTo,
		subject:           "Ative sua conta MeControla",
		tokenTTL:          tokenTTL,
		o11y:              o11y,
		dispatchedCtr:     dispatchedCtr,
	}
}

type SendActivationEmailInput struct {
	ClearToken     string
	CustomerEmail  string
	SubscriptionID string
}

func (uc *SendActivationEmail) Execute(ctx context.Context, in SendActivationEmailInput) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "onboarding.usecase.send_activation_email")
	defer span.End()

	if strings.TrimSpace(in.CustomerEmail) == "" {
		uc.dispatchedCtr.Add(ctx, 1, observability.String("result", "no_email"))
		return nil
	}
	if strings.TrimSpace(in.ClearToken) == "" {
		uc.dispatchedCtr.Add(ctx, 1, observability.String("result", "no_token"))
		return errors.New("onboarding: send activation email: token vazio")
	}

	skip, foundID, err := uc.resolveSkip(ctx, in)
	if err != nil {
		return err
	}
	if skip {
		return nil
	}

	activationURL := fmt.Sprintf("%s/ativar?token=%s", strings.TrimRight(uc.activationPageURL, "/"), in.ClearToken)
	support := fmt.Sprintf("https://wa.me/%s", sanitizeE164(uc.botNumber))
	expiresHours := int(uc.tokenTTL / time.Hour)
	if expiresHours <= 0 {
		expiresHours = 24
	}

	html, text, err := uc.template.Render(ActivationTemplateInput{
		ActivationURL:  activationURL,
		SupportURL:     support,
		ExpiresInHours: expiresHours,
	})
	if err != nil {
		return fmt.Errorf("onboarding: send activation email: render template: %w", err)
	}

	if err := uc.sender.Send(ctx, appinterfaces.EmailMessage{
		To:          in.CustomerEmail,
		Subject:     uc.subject,
		HTMLBody:    html,
		TextBody:    text,
		FromAddress: uc.fromAddress,
		FromName:    uc.fromName,
		ReplyTo:     uc.replyTo,
	}); err != nil {
		uc.dispatchedCtr.Add(ctx, 1, observability.String("result", "send_failed"))
		return fmt.Errorf("onboarding: send activation email: send: %w", err)
	}

	if foundID != "" {
		if tsErr := uc.repo.UpdateSetEmailSentAt(ctx, foundID, time.Now().UTC()); tsErr != nil {
			span.RecordError(tsErr)
		}
	}

	uc.dispatchedCtr.Add(ctx, 1, observability.String("result", "sent"))
	return nil
}

func (uc *SendActivationEmail) resolveSkip(ctx context.Context, in SendActivationEmailInput) (bool, string, error) {
	if uc.boundChecker != nil && strings.TrimSpace(in.SubscriptionID) != "" {
		bound, err := uc.boundChecker.IsAlreadyBound(ctx, in.SubscriptionID)
		if err != nil {
			return false, "", fmt.Errorf("onboarding: send activation email: check bound: %w", err)
		}
		if bound {
			uc.dispatchedCtr.Add(ctx, 1, observability.String("result", "skipped_already_bound"))
			return true, "", nil
		}
	}

	clearToken, err := valueobjects.TokenFromClear(in.ClearToken)
	if err != nil {
		return false, "", fmt.Errorf("onboarding: send activation email: parse token: %w", err)
	}

	found, findErr := uc.repo.FindByHash(ctx, clearToken.Hash())
	if findErr != nil {
		if errors.Is(findErr, domain.ErrTokenNotFound) {
			return false, "", nil
		}
		return false, "", fmt.Errorf("onboarding: send activation email: find token: %w", findErr)
	}

	st := found.Status()
	if st == valueobjects.TokenStatusConsumed || st == valueobjects.TokenStatusExpired {
		uc.dispatchedCtr.Add(ctx, 1, observability.String("result", "skipped_idempotent"))
		return true, found.ID(), nil
	}

	alreadySent, sentErr := uc.repo.IsEmailSent(ctx, found.ID())
	if sentErr != nil {
		return false, "", fmt.Errorf("onboarding: send activation email: check email sent: %w", sentErr)
	}
	if alreadySent {
		uc.dispatchedCtr.Add(ctx, 1, observability.String("result", "skipped_already_sent"))
		return true, found.ID(), nil
	}

	return false, found.ID(), nil
}
