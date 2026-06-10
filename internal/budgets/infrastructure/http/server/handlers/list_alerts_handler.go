package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
)

type listAlertsUseCase interface {
	Execute(ctx context.Context, in input.ListAlertsInput) (output.ListAlertsOutput, error)
}

type ListAlertsHandler struct {
	usecase listAlertsUseCase
	o11y    observability.Observability
}

func NewListAlertsHandler(uc listAlertsUseCase, o11y observability.Observability) *ListAlertsHandler {
	return &ListAlertsHandler{usecase: uc, o11y: o11y}
}

func (h *ListAlertsHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "budgets.handler.list_alerts")
	defer span.End()

	principal, ok := auth.FromContext(ctx)
	if !ok {
		responses.Error(w, http.StatusUnauthorized, "não autenticado")
		return
	}

	q := r.URL.Query()

	in := input.ListAlertsInput{
		UserID: principal.UserID.String(),
		Cursor: q.Get("cursor"),
	}

	if v := q.Get("limit"); v != "" {
		n, parseErr := strconv.Atoi(v)
		if parseErr != nil || n < 0 {
			span.SetAttributes(observability.String("outcome", "invalid"))
			responses.ErrorWithDetails(w, http.StatusBadRequest, "limit inválido",
				map[string]string{"code": "validation_error"})
			return
		}
		in.Limit = n
	}

	if v := q.Get("competence"); v != "" {
		comp, compErr := valueobjects.NewCompetence(v)
		if compErr != nil {
			span.SetAttributes(observability.String("outcome", "invalid"))
			responses.ErrorWithDetails(w, http.StatusBadRequest, "competence inválida",
				map[string]string{"code": "validation_error"})
			return
		}
		in.Competence = &comp
	}

	if v := q.Get("root_slug"); v != "" {
		slug, slugErr := valueobjects.ParseRootSlug(v)
		if slugErr != nil {
			span.SetAttributes(observability.String("outcome", "invalid"))
			responses.ErrorWithDetails(w, http.StatusBadRequest, "root_slug inválido",
				map[string]string{"code": "validation_error"})
			return
		}
		in.RootSlug = &slug
	}

	if v := q.Get("threshold"); v != "" {
		n, parseErr := strconv.Atoi(v)
		if parseErr != nil {
			span.SetAttributes(observability.String("outcome", "invalid"))
			responses.ErrorWithDetails(w, http.StatusBadRequest, "threshold inválido",
				map[string]string{"code": "validation_error"})
			return
		}
		t, tErr := valueobjects.ParseThreshold(n)
		if tErr != nil {
			span.SetAttributes(observability.String("outcome", "invalid"))
			responses.ErrorWithDetails(w, http.StatusBadRequest, "threshold inválido",
				map[string]string{"code": "validation_error"})
			return
		}
		in.Threshold = &t
	}

	out, err := h.usecase.Execute(ctx, in)
	if err != nil {
		span.RecordError(err)
		if errors.Is(err, usecases.ErrListAlertsInvalidUserID) {
			span.SetAttributes(observability.String("outcome", "invalid"))
			responses.ErrorWithDetails(w, http.StatusBadRequest, err.Error(),
				map[string]string{"code": "validation_error"})
			return
		}
		span.SetAttributes(observability.String("outcome", "internal_error"))
		responses.Error(w, http.StatusInternalServerError, "erro interno")
		return
	}

	span.SetAttributes(observability.String("outcome", "ok"))
	responses.JSON(w, http.StatusOK, out)
}
