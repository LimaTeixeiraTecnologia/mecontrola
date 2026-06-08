package usecases

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	application "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
)

const outreachBatchSize = 100

type SendOutreach struct {
	mgr          manager.Manager
	factory      appinterfaces.RepositoryFactory
	gateway      appinterfaces.WhatsAppGateway
	cipher       appinterfaces.TokenCipher
	idGen        id.Generator
	templateName string
	outreachGap  time.Duration
	o11y         observability.Observability
	outreachSent observability.Counter
}

func NewSendOutreach(
	mgr manager.Manager,
	factory appinterfaces.RepositoryFactory,
	gateway appinterfaces.WhatsAppGateway,
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
		mgr:          mgr,
		factory:      factory,
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
	repo := uc.factory.MagicTokenRepository(uc.mgr.DBTX(ctx))

	candidates, err := repo.FindPaidForOutreach(ctx, olderThan, outreachBatchSize)
	if err != nil {
		return fmt.Errorf("onboarding: send outreach: find candidates: %w", err)
	}

	for _, token := range candidates {
		if err := uc.sendForToken(ctx, repo, token); err != nil {
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
	repo appinterfaces.MagicTokenRepository,
	token entities.MagicToken,
) error {
	now := time.Now().UTC()

	if err := repo.UpdateMarkOutreachSent(ctx, token.ID(), now); err != nil {
		return fmt.Errorf("onboarding: send outreach: mark sent: %w", err)
	}

	clearToken, err := uc.cipher.Decrypt(ctx, token.ActivationTokenCiphertext())
	if err != nil {
		return fmt.Errorf("onboarding: send outreach: decrypt token: %w", err)
	}

	_, err = uc.gateway.SendActivationTemplate(ctx, token.CustomerMobileE164(), uc.templateName, clearToken)
	if err != nil {
		if errors.Is(err, application.ErrWhatsAppClientError) {
			uc.outreachSent.Add(ctx, 1, observability.String("result", "failed_4xx"))
			slog.WarnContext(ctx, "onboarding.outreach.failed_4xx",
				"token_id", token.ID(),
				"error", err.Error(),
				"retry_planned", false,
			)
			return fmt.Errorf("onboarding: send outreach: send template (4xx, sem reset): %w", err)
		}

		resetErr := repo.UpdateMarkOutreachReset(ctx, token.ID())
		if resetErr != nil {
			slog.WarnContext(ctx, "onboarding.outreach.reset_failed",
				"token_id", token.ID(),
				"error", resetErr.Error(),
			)
		}
		uc.outreachSent.Add(ctx, 1, observability.String("result", "failed_5xx"))
		slog.WarnContext(ctx, "onboarding.outreach.failed_5xx",
			"token_id", token.ID(),
			"error", err.Error(),
			"retry_planned", true,
		)
		return fmt.Errorf("onboarding: send outreach: send template (5xx, reset): %w", err)
	}

	uc.outreachSent.Add(ctx, 1, observability.String("result", "sent"))
	slog.InfoContext(ctx, "onboarding.outreach.sent",
		"token_id", token.ID(),
		"to_mobile_masked", maskMobile(token.CustomerMobileE164()),
	)

	return nil
}
