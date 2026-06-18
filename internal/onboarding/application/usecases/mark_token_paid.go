package usecases

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/input"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type MarkTokenPaid struct {
	repo       appinterfaces.MagicTokenRepository
	workflow   services.MagicTokenWorkflow
	o11y       observability.Observability
	tokensPaid observability.Counter
}

func NewMarkTokenPaid(
	repo appinterfaces.MagicTokenRepository,
	workflow services.MagicTokenWorkflow,
	o11y observability.Observability,
) *MarkTokenPaid {
	tokensPaid := o11y.Metrics().Counter(
		"onboarding_tokens_paid_total",
		"Total de tokens marcados como pagos",
		"1",
	)
	return &MarkTokenPaid{repo: repo, workflow: workflow, o11y: o11y, tokensPaid: tokensPaid}
}

func (uc *MarkTokenPaid) Execute(ctx context.Context, in input.MarkTokenPaidInput) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "onboarding.usecase.mark_token_paid")
	defer span.End()

	if in.FunnelToken == "" {
		return fmt.Errorf("onboarding: mark token paid: funnel token is required")
	}

	clearToken, err := valueobjects.TokenFromClear(in.FunnelToken)
	if err != nil {
		return fmt.Errorf("onboarding: mark token paid: parse funnel token: %w", err)
	}

	token, err := uc.repo.FindByHash(ctx, clearToken.Hash())
	if err != nil {
		return fmt.Errorf("onboarding: mark token paid: find token: %w", err)
	}

	decision, err := uc.workflow.DecideMarkPaid(token, services.MarkPaidCommand{
		SubscriptionID:     in.SubscriptionID,
		CustomerMobileE164: in.CustomerMobileE164,
		CustomerEmail:      in.CustomerEmail,
		ExternalSaleID:     in.ExternalSaleID,
		PaidAt:             in.PaidAt,
	})
	if err != nil {
		return fmt.Errorf("onboarding: mark token paid: decide: %w", err)
	}

	if decision.Outcome == services.MarkPaidOutcomeNoChange {
		slog.InfoContext(ctx, "onboarding.token.mark_paid.noop",
			"token_id", token.ID(),
			"token_hash_prefix", clearToken.HashPrefix(),
		)
		return nil
	}

	if err := uc.repo.UpdateMarkPaid(ctx, decision.Token); err != nil {
		return fmt.Errorf("onboarding: mark token paid: update: %w", err)
	}

	uc.tokensPaid.Add(ctx, 1)
	slog.InfoContext(ctx, "onboarding.token.marked_paid",
		"token_id", decision.Token.ID(),
		"token_hash_prefix", clearToken.HashPrefix(),
		"external_sale_id", in.ExternalSaleID,
	)

	return nil
}
