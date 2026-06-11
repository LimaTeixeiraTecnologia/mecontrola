package usecases

import (
	"context"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/services"
)

type InvoiceFor struct {
	factory interfaces.RepositoryFactory
	mgr     manager.Manager
	loc     *time.Location
	o11y    observability.Observability
}

func NewInvoiceFor(
	factory interfaces.RepositoryFactory,
	mgr manager.Manager,
	loc *time.Location,
	o11y observability.Observability,
) *InvoiceFor {
	return &InvoiceFor{factory: factory, mgr: mgr, loc: loc, o11y: o11y}
}

func (u *InvoiceFor) Execute(ctx context.Context, in input.InvoiceFor) (output.Invoice, error) {
	ctx, span := u.o11y.Tracer().Start(ctx, "card.usecase.invoice_for",
		observability.WithAttributes(
			observability.String("card_id", in.CardID.String()),
			observability.String("user_id", in.UserID.String()),
		),
	)
	defer span.End()

	if in.Purchase.IsZero() {
		span.SetAttributes(observability.String("outcome", "invalid"))
		return output.Invoice{}, domain.ErrInvalidPurchaseDate
	}

	repo := u.factory.CardRepository(u.mgr.DBTX(ctx))
	card, err := repo.GetByIDForUser(ctx, in.CardID.String(), in.UserID.String())
	if err != nil {
		outcome := classifyCardOutcome(err)
		span.RecordError(err)
		span.SetAttributes(observability.String("outcome", outcome))
		return output.Invoice{}, err
	}

	if card.IsDeleted() {
		span.SetAttributes(observability.String("outcome", "not_found"))
		return output.Invoice{}, domain.ErrCardNotFound
	}

	invoice := services.InvoiceFor(in.Purchase, card.Cycle, u.loc)

	span.SetAttributes(observability.String("outcome", "success"))
	u.o11y.Logger().Info(ctx, "card.invoice_for.computed",
		observability.String("card_id", in.CardID.String()),
		observability.String("user_id", in.UserID.String()),
	)

	return output.Invoice{
		ClosingDate: invoice.ClosingDate.In(u.loc).Format("2006-01-02"),
		DueDate:     invoice.DueDate.In(u.loc).Format("2006-01-02"),
	}, nil
}
