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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type ListCardPurchasesInput struct {
	CardID   uuid.UUID
	RefMonth *valueobjects.RefMonth
	Cursor   interfaces.Cursor
	Limit    int
}

type ListCardPurchasesOutput struct {
	Items      []output.CardPurchase
	NextCursor interfaces.Cursor
}

type ListCardPurchases struct {
	factory interfaces.RepositoryFactory
	uow     uow.UnitOfWork[ListCardPurchasesOutput]
	o11y    observability.Observability
}

func NewListCardPurchases(factory interfaces.RepositoryFactory, u uow.UnitOfWork[ListCardPurchasesOutput], o11y observability.Observability) *ListCardPurchases {
	return &ListCardPurchases{factory: factory, uow: u, o11y: o11y}
}

func (uc *ListCardPurchases) Execute(ctx context.Context, in ListCardPurchasesInput) (ListCardPurchasesOutput, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "transactions.usecase.list_card_purchases")
	defer span.End()

	principal, ok := auth.FromContext(ctx)
	if !ok || principal.IsZero() {
		return ListCardPurchasesOutput{}, ErrUsecaseUnauthorized
	}

	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}

	result, err := uc.uow.Do(ctx, func(ctx context.Context, db database.DBTX) (ListCardPurchasesOutput, error) {
		repo := uc.factory.CardPurchaseRepository(db)
		purchases, cursor, listErr := repo.ListByCardAndMonth(ctx, principal.UserID, in.CardID, in.RefMonth, in.Cursor, limit)
		if listErr != nil {
			return ListCardPurchasesOutput{}, fmt.Errorf("transactions/list_card_purchases: listar compras: %w", listErr)
		}
		items := make([]output.CardPurchase, len(purchases))
		for i, p := range purchases {
			items[i] = output.CardPurchaseFrom(p, nil, nil)
		}
		return ListCardPurchasesOutput{Items: items, NextCursor: cursor}, nil
	})
	if err != nil {
		span.RecordError(err)
		return ListCardPurchasesOutput{}, err
	}
	return result, nil
}
