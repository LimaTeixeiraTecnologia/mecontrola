package handlers

import (
	"context"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
)

type getTokenStateUseCase interface {
	Execute(ctx context.Context, clearToken string) (usecases.GetTokenStateResult, error)
}

type tokenStateResponse struct {
	ReadyToActivate  bool   `json:"ready_to_activate"`
	WaMeURL          string `json:"wa_me_url,omitempty"`
	BotNumberDisplay string `json:"bot_number_display,omitempty"`
}

type TokenStateHandler struct {
	usecase       getTokenStateUseCase
	invalidAccess func(reason string)
	o11y          observability.Observability
}

func NewTokenStateHandler(
	uc getTokenStateUseCase,
	invalidAccess func(reason string),
	o11y observability.Observability,
) *TokenStateHandler {
	return &TokenStateHandler{
		usecase:       uc,
		invalidAccess: invalidAccess,
		o11y:          o11y,
	}
}

func (h *TokenStateHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "onboarding.handler.token_state")
	defer span.End()

	token := chi.URLParam(r, "token")

	result, err := h.usecase.Execute(ctx, token)
	if err != nil {
		span.RecordError(err)
		h.o11y.Logger().Error(ctx, "onboarding.token_state.failed",
			observability.Error(err),
		)
		responses.Error(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Cache-Control", "no-store")

	if !result.Output.ReadyToActivate {
		h.invalidAccess(string(result.Reason))
		jitter := time.Duration(rand.IntN(3)) * time.Millisecond
		time.Sleep(jitter)
		responses.JSON(w, http.StatusOK, tokenStateResponse{ReadyToActivate: false})
		return
	}

	responses.JSON(w, http.StatusOK, tokenStateResponse{
		ReadyToActivate:  true,
		WaMeURL:          result.Output.WaMeURL,
		BotNumberDisplay: result.Output.BotNumberDisplay,
	})
}
