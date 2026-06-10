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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
)

type createRecurrenceUseCase interface {
	Execute(ctx context.Context, in input.CreateRecurrenceInput) (output.RecurrenceResultOutput, error)
}

type createRecurrenceRequest struct {
	SourceCompetence string `json:"source_competence"`
	Months           int    `json:"months"`
}

type CreateRecurrenceHandler struct {
	usecase createRecurrenceUseCase
	o11y    observability.Observability
}

func NewCreateRecurrenceHandler(uc createRecurrenceUseCase, o11y observability.Observability) *CreateRecurrenceHandler {
	return &CreateRecurrenceHandler{usecase: uc, o11y: o11y}
}

func (h *CreateRecurrenceHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "budgets.handler.create_recurrence")
	defer span.End()

	principal, ok := auth.FromContext(ctx)
	if !ok {
		responses.Error(w, http.StatusUnauthorized, "não autenticado")
		return
	}

	var req createRecurrenceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		span.SetAttributes(observability.String("outcome", "invalid_payload"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "payload inválido",
			map[string]string{"code": "invalid_payload"})
		return
	}

	out, err := h.usecase.Execute(ctx, input.CreateRecurrenceInput{
		UserID:           principal.UserID.String(),
		SourceCompetence: req.SourceCompetence,
		Months:           req.Months,
	})
	if err != nil {
		span.RecordError(err)
		switch {
		case errors.Is(err, usecases.ErrRecurrenceInvalidMonths),
			errors.Is(err, usecases.ErrRecurrenceSourceInvalid),
			errors.Is(err, usecases.ErrRecurrenceSourceAutoDraftWithoutAllocs),
			errors.Is(err, usecases.ErrRecurrenceSourceDraftWithoutFullAllocs),
			errors.Is(err, usecases.ErrRecurrenceSourceNegativeTotal),
			errors.Is(err, usecases.ErrBudgetInvalidCompetence):
			span.SetAttributes(observability.String("outcome", "unprocessable"))
			responses.ErrorWithDetails(w, http.StatusUnprocessableEntity, err.Error(),
				map[string]string{"code": "recurrence_invalid"})
		default:
			span.SetAttributes(observability.String("outcome", "internal_error"))
			responses.Error(w, http.StatusInternalServerError, "erro interno")
		}
		return
	}

	span.SetAttributes(observability.String("outcome", "created"))
	responses.JSON(w, http.StatusMultiStatus, out)
}
