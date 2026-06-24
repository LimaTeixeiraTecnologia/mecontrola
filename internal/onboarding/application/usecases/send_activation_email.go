package usecases

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
)

type ActivationTemplateInput struct {
	WaMeURL        string
	SupportURL     string
	ExpiresInHours int
}

type ActivationTemplate interface {
	Render(in ActivationTemplateInput) (html string, text string, err error)
}

type SendActivationEmail struct {
	sender        appinterfaces.EmailSender
	template      ActivationTemplate
	botNumber     string
	fromAddress   string
	fromName      string
	replyTo       string
	subject       string
	tokenTTL      time.Duration
	o11y          observability.Observability
	dispatchedCtr observability.Counter
}

func NewSendActivationEmail(
	sender appinterfaces.EmailSender,
	template ActivationTemplate,
	botNumber string,
	fromAddress string,
	fromName string,
	replyTo string,
	tokenTTL time.Duration,
	o11y observability.Observability,
) *SendActivationEmail {
	dispatchedCtr := o11y.Metrics().Counter(
		"onboarding_activation_email_dispatched_total",
		"Total de emails de ativacao enviados",
		"1",
	)
	return &SendActivationEmail{
		sender:        sender,
		template:      template,
		botNumber:     botNumber,
		fromAddress:   fromAddress,
		fromName:      fromName,
		replyTo:       replyTo,
		subject:       "Ative sua conta MeControla",
		tokenTTL:      tokenTTL,
		o11y:          o11y,
		dispatchedCtr: dispatchedCtr,
	}
}

type SendActivationEmailInput struct {
	ClearToken    string
	CustomerEmail string
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

	waMe := fmt.Sprintf("https://wa.me/%s?text=ATIVAR%%20%s", sanitizeE164(uc.botNumber), in.ClearToken)
	support := fmt.Sprintf("https://wa.me/%s", sanitizeE164(uc.botNumber))
	expiresHours := int(uc.tokenTTL / time.Hour)
	if expiresHours <= 0 {
		expiresHours = 24
	}

	html, text, err := uc.template.Render(ActivationTemplateInput{
		WaMeURL:        waMe,
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

	uc.dispatchedCtr.Add(ctx, 1, observability.String("result", "sent"))
	return nil
}
