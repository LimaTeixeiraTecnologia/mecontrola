package handlers

import (
	"context"
	"net/http"
	"strconv"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
)

const (
	defaultLimit = 20
	maxLimit     = 100
)

type listCardsUseCase interface {
	Execute(ctx context.Context, in input.ListCards) (output.CardList, error)
}

type ListCardsHandler struct {
	usecase listCardsUseCase
	o11y    observability.Observability
}

func NewListCardsHandler(uc listCardsUseCase, o11y observability.Observability) *ListCardsHandler {
	return &ListCardsHandler{usecase: uc, o11y: o11y}
}

func (h *ListCardsHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "card.handler.list")
	defer span.End()

	principal, _ := auth.FromContext(ctx)

	cursor := r.URL.Query().Get("cursor")

	limit := defaultLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			span.SetAttributes(observability.String("outcome", "invalid"))
			responses.ErrorWithDetails(w, http.StatusBadRequest, "parâmetro limit inválido",
				map[string]string{"code": "invalid_limit"})
			return
		}
		if parsed > maxLimit {
			parsed = maxLimit
		}
		limit = parsed
	}

	span.SetAttributes(observability.String("user_id", principal.UserID.String()))

	out, err := h.usecase.Execute(ctx, input.ListCards{
		UserID: principal.UserID,
		Cursor: cursor,
		Limit:  limit,
	})
	if err != nil {
		span.RecordError(err)
		mapCardError(w, span, err)
		return
	}

	span.SetAttributes(observability.String("outcome", "success"))
	h.o11y.Logger().Info(ctx, "card.list.served",
		observability.String("user_id", principal.UserID.String()),
		observability.Int("count", len(out.Items)),
	)

	responses.JSON(w, http.StatusOK, out)
}
