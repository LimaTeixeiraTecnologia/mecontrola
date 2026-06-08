package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/input"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
)

type HandlePaidWithoutToken struct {
	mgr              manager.Manager
	factory          appinterfaces.RepositoryFactory
	idGen            id.Generator
	o11y             observability.Observability
	paidWithoutToken observability.Counter
}

func NewHandlePaidWithoutToken(
	mgr manager.Manager,
	factory appinterfaces.RepositoryFactory,
	idGen id.Generator,
	o11y observability.Observability,
) *HandlePaidWithoutToken {
	paidWithoutToken := o11y.Metrics().Counter(
		"billing_paid_without_token_total",
		"Total de pagamentos recebidos sem token de funil",
		"1",
	)
	return &HandlePaidWithoutToken{mgr: mgr, factory: factory, idGen: idGen, o11y: o11y, paidWithoutToken: paidWithoutToken}
}

func (uc *HandlePaidWithoutToken) Execute(ctx context.Context, in input.HandlePaidWithoutTokenInput) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "onboarding.usecase.handle_paid_without_token")
	defer span.End()

	payload, err := json.Marshal(map[string]any{
		"external_sale_id":       in.ExternalSaleID,
		"customer_mobile_masked": maskMobile(in.CustomerMobileE164),
		"customer_email_masked":  maskEmail(in.CustomerEmail),
		"paid_at":                in.PaidAt,
	})
	if err != nil {
		return fmt.Errorf("onboarding: handle paid without token: marshal signal: %w", err)
	}

	sig, err := entities.NewSupportSignal(uc.idGen.NewID(), valueobjects.SupportSignalKindPaidWithoutToken, payload)
	if err != nil {
		return fmt.Errorf("onboarding: handle paid without token: new signal: %w", err)
	}

	repo := uc.factory.SupportSignalRepository(uc.mgr.DBTX(ctx))
	if err := repo.Insert(ctx, sig); err != nil {
		return fmt.Errorf("onboarding: handle paid without token: insert signal: %w", err)
	}

	uc.paidWithoutToken.Add(ctx, 1, observability.String("provider", "kiwify"))
	slog.WarnContext(ctx, "onboarding.paid_without_token",
		"external_sale_id", in.ExternalSaleID,
		"customer_mobile_masked", maskMobile(in.CustomerMobileE164),
	)

	return nil
}

func maskEmail(email string) string {
	if email == "" {
		return ""
	}
	for i, c := range email {
		if c == '@' && i > 0 {
			return email[:1] + "****" + email[i:]
		}
	}
	return "****"
}
