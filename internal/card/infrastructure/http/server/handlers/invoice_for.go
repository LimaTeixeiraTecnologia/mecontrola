package handlers

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
)

type invoiceForUseCase interface {
	Execute(ctx context.Context, in input.InvoiceFor) (output.Invoice, error)
}

type InvoiceForHandler struct {
	usecase invoiceForUseCase
	o11y    observability.Observability
}

func NewInvoiceForHandler(uc invoiceForUseCase, o11y observability.Observability) *InvoiceForHandler {
	return &InvoiceForHandler{usecase: uc, o11y: o11y}
}

func (h *InvoiceForHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "card.handler.invoice_for")
	defer span.End()

	principal, _ := auth.FromContext(ctx)

	cardID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		span.SetAttributes(observability.String("outcome", "invalid"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "id do cartão inválido",
			map[string]string{"code": "invalid_card_id"})
		return
	}

	span.SetAttributes(
		observability.String("card_id", cardID.String()),
		observability.String("user_id", principal.UserID.String()),
	)

	forParam := r.URL.Query().Get("for")
	if forParam == "" {
		span.SetAttributes(observability.String("outcome", "invalid"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "parâmetro 'for' obrigatório",
			map[string]string{"code": "missing_for_param"})
		return
	}

	in, err := input.NewInvoiceFor(cardID, principal.UserID, forParam)
	if err != nil {
		span.SetAttributes(observability.String("outcome", "invalid"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "data inválida; use YYYY-MM-DD",
			map[string]string{"code": "invalid_purchase_date"})
		return
	}

	out, err := h.usecase.Execute(ctx, in)
	if err != nil {
		span.RecordError(err)
		mapCardError(w, span, err)
		return
	}

	span.SetAttributes(observability.String("outcome", "success"))
	h.o11y.Logger().Info(ctx, "card.invoice_for.computed",
		observability.String("card_id", cardID.String()),
		observability.String("user_id", principal.UserID.String()),
	)

	responses.JSON(w, http.StatusOK, out)
}
