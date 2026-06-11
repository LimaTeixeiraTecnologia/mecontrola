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

	in := input.ListAlertsInput{
		UserID: principal.UserID.String(),
		Cursor: r.URL.Query().Get("cursor"),
	}

	if message, invalid := h.applyLimit(r, &in); invalid {
		span.SetAttributes(observability.String("outcome", "invalid"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, message, map[string]string{"code": "validation_error"})
		return
	}

	if message, invalid := h.applyCompetence(r, &in); invalid {
		span.SetAttributes(observability.String("outcome", "invalid"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, message, map[string]string{"code": "validation_error"})
		return
	}

	if message, invalid := h.applyRootSlug(r, &in); invalid {
		span.SetAttributes(observability.String("outcome", "invalid"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, message, map[string]string{"code": "validation_error"})
		return
	}

	if message, invalid := h.applyThreshold(r, &in); invalid {
		span.SetAttributes(observability.String("outcome", "invalid"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, message, map[string]string{"code": "validation_error"})
		return
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

func (h *ListAlertsHandler) applyLimit(r *http.Request, in *input.ListAlertsInput) (string, bool) {
	value := r.URL.Query().Get("limit")
	if value == "" {
		return "", false
	}

	limit, err := strconv.Atoi(value)
	if err != nil || limit < 0 {
		return "limit inválido", true
	}

	in.Limit = limit
	return "", false
}

func (h *ListAlertsHandler) applyCompetence(r *http.Request, in *input.ListAlertsInput) (string, bool) {
	value := r.URL.Query().Get("competence")
	if value == "" {
		return "", false
	}

	competence, err := valueobjects.NewCompetence(value)
	if err != nil {
		return "competence inválida", true
	}

	in.Competence = &competence
	return "", false
}

func (h *ListAlertsHandler) applyRootSlug(r *http.Request, in *input.ListAlertsInput) (string, bool) {
	value := r.URL.Query().Get("root_slug")
	if value == "" {
		return "", false
	}

	rootSlug, err := valueobjects.ParseRootSlug(value)
	if err != nil {
		return "root_slug inválido", true
	}

	in.RootSlug = &rootSlug
	return "", false
}

func (h *ListAlertsHandler) applyThreshold(r *http.Request, in *input.ListAlertsInput) (string, bool) {
	value := r.URL.Query().Get("threshold")
	if value == "" {
		return "", false
	}

	rawThreshold, err := strconv.Atoi(value)
	if err != nil {
		return "threshold inválido", true
	}

	threshold, err := valueobjects.ParseThreshold(rawThreshold)
	if err != nil {
		return "threshold inválido", true
	}

	in.Threshold = &threshold
	return "", false
}
