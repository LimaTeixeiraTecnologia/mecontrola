package usecases

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type GetCardInvoice struct {
	factory interfaces.RepositoryFactory
	uow     uow.UnitOfWork
	o11y    observability.Observability
}

func NewGetCardInvoice(factory interfaces.RepositoryFactory, u uow.UnitOfWork, o11y observability.Observability) *GetCardInvoice {
	return &GetCardInvoice{factory: factory, uow: u, o11y: o11y}
}

func (uc *GetCardInvoice) Execute(ctx context.Context, cardID uuid.UUID, refMonthStr string) (output.CardInvoice, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "transactions.usecase.get_card_invoice")
	defer span.End()

	principal, ok := auth.FromContext(ctx)
	if !ok || principal.IsZero() {
		return output.CardInvoice{}, ErrUsecaseUnauthorized
	}

	refMonth, err := valueobjects.NewRefMonth(refMonthStr)
	if err != nil {
		return output.CardInvoice{}, fmt.Errorf("transactions/get_card_invoice: ref_month inválido: %w", err)
	}

	result, execErr := uow.Do(ctx, uc.uow, func(ctx context.Context, db database.DBTX) (output.CardInvoice, error) {
		invoicesRepo := uc.factory.CardInvoiceRepository(db)
		inv, items, getErr := invoicesRepo.GetByMonth(ctx, principal.UserID, cardID, refMonth)
		if getErr != nil {
			return output.CardInvoice{}, fmt.Errorf("transactions/get_card_invoice: obter fatura: %w", getErr)
		}
		if inv == nil {
			return output.CardInvoice{}, ErrCardInvoiceNotFound
		}
		return output.CardInvoiceFrom(inv, items), nil
	})
	if execErr != nil {
		span.RecordError(execErr)
		return output.CardInvoice{}, execErr
	}
	return result, nil
}
