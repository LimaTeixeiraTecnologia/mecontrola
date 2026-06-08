package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/output"
)

type createCheckoutSessionUseCase interface {
	Execute(ctx context.Context, in input.CreateCheckoutSessionInput) (output.CreateCheckoutSessionOutput, error)
}

type checkoutRequest struct {
	PlanID string `json:"plan_id"`
}

type checkoutResponse struct {
	CheckoutURL string `json:"checkout_url"`
}

type CreateCheckoutHandler struct {
	usecase             createCheckoutSessionUseCase
	checkoutCreated     func(planID string)
	checkoutRateLimited func()
	o11y                observability.Observability
}

func NewCreateCheckoutHandler(
	uc createCheckoutSessionUseCase,
	checkoutCreated func(planID string),
	checkoutRateLimited func(),
	o11y observability.Observability,
) *CreateCheckoutHandler {
	return &CreateCheckoutHandler{
		usecase:             uc,
		checkoutCreated:     checkoutCreated,
		checkoutRateLimited: checkoutRateLimited,
		o11y:                o11y,
	}
}

func (h *CreateCheckoutHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "onboarding.handler.create_checkout")
	defer span.End()

	var req checkoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responses.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.PlanID == "" {
		responses.Error(w, http.StatusBadRequest, "plan_id is required")
		return
	}

	result, err := h.usecase.Execute(ctx, input.CreateCheckoutSessionInput{PlanID: req.PlanID})
	if err != nil {
		span.RecordError(err)
		switch {
		case errors.Is(err, application.ErrUnknownPlan):
			responses.ErrorWithDetails(w, http.StatusBadRequest, "unknown plan",
				map[string]string{"code": "unknown_plan"})
		case errors.Is(err, application.ErrCheckoutUnavailable):
			responses.ErrorWithDetails(w, http.StatusServiceUnavailable, "checkout unavailable",
				map[string]string{"code": "checkout_unavailable"})
		default:
			h.o11y.Logger().Error(ctx, "onboarding.checkout.create_failed",
				observability.Error(err),
			)
			responses.Error(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	h.checkoutCreated(req.PlanID)
	responses.JSON(w, http.StatusCreated, checkoutResponse{CheckoutURL: result.CheckoutURL})
}
