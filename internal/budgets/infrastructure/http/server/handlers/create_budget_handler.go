package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
)

type createBudgetUseCase interface {
	Execute(ctx context.Context, in input.CreateBudgetInput) (output.BudgetOutput, error)
}

type createBudgetRequest struct {
	Competence  string                   `json:"competence"`
	TotalCents  int64                    `json:"total_cents"`
	Allocations []allocationRequestEntry `json:"allocations"`
}

type allocationRequestEntry struct {
	RootSlug    string `json:"root_slug"`
	BasisPoints int    `json:"basis_points"`
}

type CreateBudgetHandler struct {
	usecase createBudgetUseCase
	o11y    observability.Observability
}

func NewCreateBudgetHandler(uc createBudgetUseCase, o11y observability.Observability) *CreateBudgetHandler {
	return &CreateBudgetHandler{usecase: uc, o11y: o11y}
}

func (h *CreateBudgetHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "budgets.handler.create_budget")
	defer span.End()

	principal, ok := auth.FromContext(ctx)
	if !ok {
		responses.Error(w, http.StatusUnauthorized, "não autenticado")
		return
	}

	var req createBudgetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		span.SetAttributes(observability.String("outcome", "invalid_payload"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "payload inválido",
			map[string]string{"code": "invalid_payload"})
		return
	}

	allocs := make([]input.AllocationInput, 0, len(req.Allocations))
	for _, a := range req.Allocations {
		allocs = append(allocs, input.AllocationInput{
			RootSlug:    a.RootSlug,
			BasisPoints: a.BasisPoints,
		})
	}

	out, err := h.usecase.Execute(ctx, input.CreateBudgetInput{
		UserID:      principal.UserID.String(),
		Competence:  req.Competence,
		TotalCents:  req.TotalCents,
		Allocations: allocs,
	})
	if err != nil {
		span.RecordError(err)
		mapBudgetError(w, span, err)
		return
	}

	span.SetAttributes(
		observability.String("budget_id", out.ID),
		observability.String("outcome", "created"),
	)
	w.Header().Set("Location", "/api/v1/budgets/"+req.Competence)
	responses.JSON(w, http.StatusCreated, out)
}

func mapBudgetError(w http.ResponseWriter, span observability.Span, err error) {
	switch {
	case errors.Is(err, interfaces.ErrBudgetConflict):
		span.SetAttributes(observability.String("outcome", "conflict"))
		responses.ErrorWithDetails(w, http.StatusConflict, "orçamento já existe para esta competência",
			map[string]string{"code": "budget_conflict"})
	case errors.Is(err, interfaces.ErrBudgetNotFound):
		span.SetAttributes(observability.String("outcome", "not_found"))
		responses.ErrorWithDetails(w, http.StatusNotFound, "orçamento não encontrado",
			map[string]string{"code": "budget_not_found"})
	case errors.Is(err, usecases.ErrBudgetInvalidCompetence),
		errors.Is(err, usecases.ErrBudgetInvalidTotalCents),
		errors.Is(err, usecases.ErrBudgetInvalidAllocationRootSlug),
		errors.Is(err, usecases.ErrBudgetAllocationBasisPointsInvalid),
		errors.Is(err, usecases.ErrBudgetAllocationSumExceeds10000),
		errors.Is(err, usecases.ErrBudgetInvalidUserID):
		span.SetAttributes(observability.String("outcome", "invalid"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, err.Error(),
			map[string]string{"code": "validation_error"})
	case errors.Is(err, usecases.ErrRecurrenceInvalidMonths),
		errors.Is(err, usecases.ErrRecurrenceSourceInvalid):
		span.SetAttributes(observability.String("outcome", "invalid"))
		responses.ErrorWithDetails(w, http.StatusUnprocessableEntity, err.Error(),
			map[string]string{"code": "validation_error"})
	default:
		span.SetAttributes(observability.String("outcome", "internal_error"))
		responses.Error(w, http.StatusInternalServerError, "erro interno")
	}
}
