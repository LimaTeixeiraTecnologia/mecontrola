package usecases

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
)

type GetCardPurchase struct {
	factory interfaces.RepositoryFactory
	uow     uow.UnitOfWork[output.CardPurchase]
	o11y    observability.Observability
}

func NewGetCardPurchase(factory interfaces.RepositoryFactory, u uow.UnitOfWork[output.CardPurchase], o11y observability.Observability) *GetCardPurchase {
	return &GetCardPurchase{factory: factory, uow: u, o11y: o11y}
}

func (uc *GetCardPurchase) Execute(ctx context.Context, purchaseID uuid.UUID) (output.CardPurchase, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "transactions.usecase.get_card_purchase")
	defer span.End()

	principal, ok := auth.FromContext(ctx)
	if !ok || principal.IsZero() {
		return output.CardPurchase{}, ErrUsecaseUnauthorized
	}

	result, err := uc.uow.Do(ctx, func(ctx context.Context, db database.DBTX) (output.CardPurchase, error) {
		purchasesRepo := uc.factory.CardPurchaseRepository(db)
		purchase, getErr := purchasesRepo.GetByID(ctx, purchaseID, principal.UserID)
		if getErr != nil {
			return output.CardPurchase{}, fmt.Errorf("transactions/get_card_purchase: obter compra: %w", getErr)
		}
		return output.CardPurchaseFrom(purchase, nil, nil), nil
	})
	if err != nil {
		span.RecordError(err)
		return output.CardPurchase{}, err
	}
	return result, nil
}
