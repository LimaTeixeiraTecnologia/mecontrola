package usecases

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/input"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type MarkTokenPaid struct {
	mgr        manager.Manager
	factory    appinterfaces.RepositoryFactory
	o11y       observability.Observability
	tokensPaid observability.Counter
}

func NewMarkTokenPaid(
	mgr manager.Manager,
	factory appinterfaces.RepositoryFactory,
	o11y observability.Observability,
) *MarkTokenPaid {
	tokensPaid := o11y.Metrics().Counter(
		"onboarding_tokens_paid_total",
		"Total de tokens marcados como pagos",
		"1",
	)
	return &MarkTokenPaid{mgr: mgr, factory: factory, o11y: o11y, tokensPaid: tokensPaid}
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

	repo := uc.factory.MagicTokenRepository(uc.mgr.DBTX(ctx))

	token, err := repo.FindByHash(ctx, clearToken.Hash())
	if err != nil {
		return fmt.Errorf("onboarding: mark token paid: find token: %w", err)
	}

	updated, err := token.MarkPaid(in.SubscriptionID, in.CustomerMobileE164, in.CustomerEmail, in.ExternalSaleID, in.PaidAt)
	if err != nil {
		return fmt.Errorf("onboarding: mark token paid: transition: %w", err)
	}

	if updated.Status() == token.Status() {
		slog.InfoContext(ctx, "onboarding.token.mark_paid.noop",
			"token_hash_prefix", token.ID(),
		)
		return nil
	}

	if err := repo.UpdateMarkPaid(ctx, updated); err != nil {
		return fmt.Errorf("onboarding: mark token paid: update: %w", err)
	}

	uc.tokensPaid.Add(ctx, 1)
	slog.InfoContext(ctx, "onboarding.token.marked_paid",
		"token_hash_prefix", updated.ID(),
		"external_sale_id", in.ExternalSaleID,
	)

	return nil
}
