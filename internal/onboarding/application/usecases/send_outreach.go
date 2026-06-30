package usecases

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	application "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
)

const outreachBatchSize = 100

const outreachChannelWhatsApp = "whatsapp"

type SendOutreach struct {
	repo         appinterfaces.MagicTokenRepository
	gateway      appinterfaces.OutreachChannelGateway
	cipher       appinterfaces.TokenCipher
	idGen        id.Generator
	templateName string
	outreachGap  time.Duration
	o11y         observability.Observability
	outreachSent observability.Counter
}

func NewSendOutreach(
	repo appinterfaces.MagicTokenRepository,
	gateway appinterfaces.OutreachChannelGateway,
	cipher appinterfaces.TokenCipher,
	idGen id.Generator,
	templateName string,
	outreachGap time.Duration,
	o11y observability.Observability,
) *SendOutreach {
	outreachSent := o11y.Metrics().Counter(
		"onboarding_outreach_sent_total",
		"Total de mensagens de outreach enviadas",
		"1",
	)
	return &SendOutreach{
		repo:         repo,
		gateway:      gateway,
		cipher:       cipher,
		idGen:        idGen,
		templateName: templateName,
		outreachGap:  outreachGap,
		o11y:         o11y,
		outreachSent: outreachSent,
	}
}

func (uc *SendOutreach) Execute(ctx context.Context) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "onboarding.usecase.send_outreach")
	defer span.End()

	olderThan := time.Now().UTC().Add(-uc.outreachGap)

	candidates, err := uc.repo.FindPaidForOutreach(ctx, olderThan, outreachBatchSize)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("onboarding: send outreach: find candidates: %w", err)
	}

	for _, token := range candidates {
		if err := uc.sendForToken(ctx, token); err != nil {
			slog.WarnContext(ctx, "onboarding.outreach.failed",
				"token_id", token.ID(),
				"error", err.Error(),
			)
		}
	}

	return nil
}

func (uc *SendOutreach) sendForToken(
	ctx context.Context,
	token entities.MagicToken,
) error {
	if token.CustomerMobileE164() == "" {
		slog.WarnContext(ctx, "onboarding.outreach.skipped_no_channel", "token_id", token.ID())
		return nil
	}

	now := time.Now().UTC()
	if err := uc.repo.UpdateMarkOutreachSent(ctx, token.ID(), now); err != nil {
		return fmt.Errorf("onboarding: send outreach: mark sent: %w", err)
	}

	clearToken, err := uc.cipher.Decrypt(ctx, token.ActivationTokenCiphertext())
	if err != nil {
		return fmt.Errorf("onboarding: send outreach: decrypt token: %w", err)
	}

	return uc.sendWhatsApp(ctx, token, clearToken)
}

func (uc *SendOutreach) sendWhatsApp(
	ctx context.Context,
	token entities.MagicToken,
	clearToken string,
) error {
	_, err := uc.gateway.SendActivationTemplate(ctx, outreachChannelWhatsApp, token.CustomerMobileE164(), uc.templateName, clearToken)
	if err != nil {
		if errors.Is(err, application.ErrWhatsAppClientError) {
			uc.outreachSent.Add(ctx, 1,
				observability.String("result", "failed_4xx"),
				observability.String("channel", outreachChannelWhatsApp),
			)
			slog.WarnContext(ctx, "onboarding.outreach.failed_4xx",
				"token_id", token.ID(),
				"channel", outreachChannelWhatsApp,
				"error", err.Error(),
				"retry_planned", false,
			)
			return fmt.Errorf("onboarding: send outreach: send template (4xx, sem reset): %w", err)
		}

		if resetErr := uc.repo.UpdateMarkOutreachReset(ctx, token.ID()); resetErr != nil {
			slog.WarnContext(ctx, "onboarding.outreach.reset_failed",
				"token_id", token.ID(),
				"channel", outreachChannelWhatsApp,
				"error", resetErr.Error(),
			)
		}
		uc.outreachSent.Add(ctx, 1,
			observability.String("result", "failed_5xx"),
			observability.String("channel", outreachChannelWhatsApp),
		)
		slog.WarnContext(ctx, "onboarding.outreach.failed_5xx",
			"token_id", token.ID(),
			"channel", outreachChannelWhatsApp,
			"error", err.Error(),
			"retry_planned", true,
		)
		return fmt.Errorf("onboarding: send outreach: send template (5xx, reset): %w", err)
	}

	uc.outreachSent.Add(ctx, 1,
		observability.String("result", "sent"),
		observability.String("channel", outreachChannelWhatsApp),
	)
	slog.InfoContext(ctx, "onboarding.outreach.sent",
		"token_id", token.ID(),
		"channel", outreachChannelWhatsApp,
		"to_mobile_masked", maskMobile(token.CustomerMobileE164()),
	)
	return nil
}
