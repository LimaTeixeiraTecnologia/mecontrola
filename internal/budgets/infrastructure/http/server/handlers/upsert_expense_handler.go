package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"
	"github.com/go-chi/chi/v5"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
)

type upsertExpenseUseCase interface {
	Execute(ctx context.Context, in input.UpsertExpenseInput) (output.ExpenseOutput, error)
}

type upsertExpenseRequest struct {
	ExternalTransactionID string     `json:"external_transaction_id"`
	SubcategoryID         string     `json:"subcategory_id"`
	Competence            string     `json:"competence"`
	AmountCents           int64      `json:"amount_cents"`
	OccurredAt            *time.Time `json:"occurred_at"`
	ExpectedVersion       *int64     `json:"expected_version"`
}

type UpsertExpenseHandler struct {
	usecase upsertExpenseUseCase
	o11y    observability.Observability
}

func NewUpsertExpenseHandler(uc upsertExpenseUseCase, o11y observability.Observability) *UpsertExpenseHandler {
	return &UpsertExpenseHandler{usecase: uc, o11y: o11y}
}

func (h *UpsertExpenseHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "budgets.handler.create_expense")
	defer span.End()

	principal, ok := auth.FromContext(ctx)
	if !ok {
		responses.Error(w, http.StatusUnauthorized, "não autenticado")
		return
	}

	var req upsertExpenseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		span.SetAttributes(observability.String("outcome", "invalid_payload"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "payload inválido",
			map[string]string{"code": "invalid_payload"})
		return
	}

	var occurredAt time.Time
	if req.OccurredAt != nil {
		occurredAt = *req.OccurredAt
	}

	out, err := h.usecase.Execute(ctx, input.UpsertExpenseInput{
		UserID:                principal.UserID.String(),
		Source:                "api",
		ExternalTransactionID: req.ExternalTransactionID,
		SubcategoryID:         req.SubcategoryID,
		Competence:            req.Competence,
		AmountCents:           req.AmountCents,
		OccurredAt:            occurredAt,
		ExpectedVersion:       nil,
	})
	if err != nil {
		span.RecordError(err)
		mapExpenseError(w, span, err)
		return
	}

	span.SetAttributes(observability.String("outcome", "created"))
	responses.JSON(w, http.StatusCreated, out)
}

func (h *UpsertExpenseHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "budgets.handler.update_expense")
	defer span.End()

	principal, ok := auth.FromContext(ctx)
	if !ok {
		responses.Error(w, http.StatusUnauthorized, "não autenticado")
		return
	}

	extID := chi.URLParam(r, "id")

	var req upsertExpenseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		span.SetAttributes(observability.String("outcome", "invalid_payload"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "payload inválido",
			map[string]string{"code": "invalid_payload"})
		return
	}

	if req.ExpectedVersion == nil {
		span.SetAttributes(observability.String("outcome", "invalid"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "expected_version obrigatório para edição",
			map[string]string{"code": "version_required"})
		return
	}

	var occurredAt time.Time
	if req.OccurredAt != nil {
		occurredAt = *req.OccurredAt
	}

	out, err := h.usecase.Execute(ctx, input.UpsertExpenseInput{
		UserID:                principal.UserID.String(),
		Source:                "api",
		ExternalTransactionID: extID,
		SubcategoryID:         req.SubcategoryID,
		Competence:            req.Competence,
		AmountCents:           req.AmountCents,
		OccurredAt:            occurredAt,
		ExpectedVersion:       req.ExpectedVersion,
	})
	if err != nil {
		span.RecordError(err)
		mapExpenseError(w, span, err)
		return
	}

	span.SetAttributes(observability.String("outcome", "updated"))
	responses.JSON(w, http.StatusOK, out)
}

func mapExpenseError(w http.ResponseWriter, span observability.Span, err error) {
	switch {
	case errors.Is(err, interfaces.ErrExpenseNotFound):
		span.SetAttributes(observability.String("outcome", "not_found"))
		responses.ErrorWithDetails(w, http.StatusNotFound, "despesa não encontrada",
			map[string]string{"code": "expense_not_found"})
	case errors.Is(err, interfaces.ErrExpenseConflict):
		span.SetAttributes(observability.String("outcome", "conflict"))
		responses.ErrorWithDetails(w, http.StatusConflict, "conflito de versão na despesa",
			map[string]string{"code": "expense_version_conflict"})
	case errors.Is(err, interfaces.ErrExpenseTombstoneConflict):
		span.SetAttributes(observability.String("outcome", "conflict"))
		responses.ErrorWithDetails(w, http.StatusConflict, "identidade canônica bloqueada por tombstone",
			map[string]string{"code": "expense_tombstone_conflict"})
	case errors.Is(err, usecases.ErrUpsertExpenseInvalidSubcategory),
		errors.Is(err, usecases.ErrUpsertExpenseInvalidExternalID),
		errors.Is(err, usecases.ErrUpsertExpenseInvalidCompetence),
		errors.Is(err, usecases.ErrUpsertExpenseInvalidSource),
		errors.Is(err, usecases.ErrUpsertExpenseInvalidUserID),
		errors.Is(err, usecases.ErrUpsertExpenseInvalidAmount),
		errors.Is(err, usecases.ErrUpsertExpenseExplicitVersion):
		span.SetAttributes(observability.String("outcome", "invalid"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, err.Error(),
			map[string]string{"code": "validation_error"})
	case errors.Is(err, usecases.ErrUpsertExpenseVersionRequired):
		span.SetAttributes(observability.String("outcome", "invalid"))
		responses.ErrorWithDetails(w, http.StatusUnprocessableEntity, err.Error(),
			map[string]string{"code": "version_required"})
	default:
		span.SetAttributes(observability.String("outcome", "internal_error"))
		responses.Error(w, http.StatusInternalServerError, "erro interno")
	}
}
