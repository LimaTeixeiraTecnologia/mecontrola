package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/output"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
)

type CreateCheckoutSession struct {
	uow     uow.UnitOfWork
	factory appinterfaces.RepositoryFactory
	builder appinterfaces.CheckoutURLBuilder
	cipher  appinterfaces.TokenCipher
	idGen   id.Generator
	ttl     time.Duration
	o11y    observability.Observability
}

func NewCreateCheckoutSession(
	u uow.UnitOfWork,
	factory appinterfaces.RepositoryFactory,
	builder appinterfaces.CheckoutURLBuilder,
	cipher appinterfaces.TokenCipher,
	idGen id.Generator,
	ttl time.Duration,
	o11y observability.Observability,
) *CreateCheckoutSession {
	return &CreateCheckoutSession{
		uow:     u,
		factory: factory,
		builder: builder,
		cipher:  cipher,
		idGen:   idGen,
		ttl:     ttl,
		o11y:    o11y,
	}
}

func (uc *CreateCheckoutSession) Execute(ctx context.Context, in input.CreateCheckoutSessionInput) (output.CreateCheckoutSessionOutput, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "onboarding.usecase.create_checkout_session")
	defer span.End()

	if in.PlanID == "" {
		return output.CreateCheckoutSessionOutput{}, fmt.Errorf("onboarding: create checkout session: plan id is required")
	}

	token, err := valueobjects.NewToken()
	if err != nil {
		return output.CreateCheckoutSessionOutput{}, fmt.Errorf("onboarding: create checkout session: generate token: %w", err)
	}

	checkoutURL, err := uc.builder.Build(ctx, in.PlanID, token.ClearText())
	if err != nil {
		return output.CreateCheckoutSessionOutput{}, fmt.Errorf("onboarding: create checkout session: build url: %w", err)
	}

	tokenID := uc.idGen.NewID()
	expiresAt := time.Now().UTC().Add(uc.ttl)

	magicToken, err := entities.NewMagicToken(tokenID, token.Hash(), in.PlanID, expiresAt)
	if err != nil {
		return output.CreateCheckoutSessionOutput{}, fmt.Errorf("onboarding: create checkout session: build entity: %w", err)
	}
	ciphertext, err := uc.cipher.Encrypt(ctx, token.ClearText())
	if err != nil {
		return output.CreateCheckoutSessionOutput{}, fmt.Errorf("onboarding: create checkout session: encrypt token: %w", err)
	}
	magicToken, err = magicToken.WithActivationTokenCiphertext(ciphertext)
	if err != nil {
		return output.CreateCheckoutSessionOutput{}, fmt.Errorf("onboarding: create checkout session: set encrypted token: %w", err)
	}

	err = uc.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) error {
		repo := uc.factory.MagicTokenRepository(tx)
		if insertErr := repo.Insert(ctx, magicToken); insertErr != nil {
			return fmt.Errorf("onboarding: create checkout session: insert: %w", insertErr)
		}
		return nil
	})
	if err != nil {
		return output.CreateCheckoutSessionOutput{}, err
	}

	return output.CreateCheckoutSessionOutput{
		CheckoutURL: checkoutURL,
		TokenID:     tokenID,
	}, nil
}
