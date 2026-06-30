package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/input"
)

type recordJourneyTimestampUseCase interface {
	Execute(ctx context.Context, in input.RecordJourneyTimestampInput) error
}

type journeyBeaconRequest struct {
	Event string `json:"event"`
}

type RecordJourneyBeaconHandler struct {
	usecase recordJourneyTimestampUseCase
	o11y    observability.Observability
}

func NewRecordJourneyBeaconHandler(
	uc recordJourneyTimestampUseCase,
	o11y observability.Observability,
) *RecordJourneyBeaconHandler {
	return &RecordJourneyBeaconHandler{usecase: uc, o11y: o11y}
}

func (h *RecordJourneyBeaconHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "onboarding.handler.record_journey_beacon")
	defer span.End()

	token := chi.URLParam(r, "token")

	var body journeyBeaconRequest
	_ = json.NewDecoder(r.Body).Decode(&body)

	if err := h.usecase.Execute(ctx, input.RecordJourneyTimestampInput{
		ClearToken: token,
		Event:      body.Event,
	}); err != nil {
		span.RecordError(err)
		h.o11y.Logger().Error(ctx, "onboarding.record_journey_beacon.failed",
			observability.Error(err),
		)
	}

	w.WriteHeader(http.StatusNoContent)
}
