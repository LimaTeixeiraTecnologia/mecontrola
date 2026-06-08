package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/http/server/handlers"
)

type stubGetTokenState struct {
	result usecases.GetTokenStateResult
	err    error
}

func (s *stubGetTokenState) Execute(_ context.Context, _ string) (usecases.GetTokenStateResult, error) {
	return s.result, s.err
}

func buildStateHandler(stub *stubGetTokenState) http.Handler {
	o11y := noop.NewProvider()
	invalidCount := 0
	h := handlers.NewTokenStateHandler(
		stub,
		func(_ string) { invalidCount++ },
		o11y,
	)

	r := chi.NewRouter()
	r.Get("/v1/onboarding/tokens/{token}/state", h.Handle)
	return r
}

func TestTokenStateHandler_200_ReadyToActivate(t *testing.T) {
	stub := &stubGetTokenState{
		result: usecases.GetTokenStateResult{
			Output: output.GetTokenStateOutput{
				ReadyToActivate:  true,
				WaMeURL:          "https://wa.me/5511999999999?text=ATIVAR%20tok123",
				BotNumberDisplay: "+55 11 9XXXX-XXXX",
			},
		},
	}
	handler := buildStateHandler(stub)

	req := httptest.NewRequest(http.MethodGet, "/v1/onboarding/tokens/some-token/state", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "no-store", rr.Header().Get("Cache-Control"))

	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, true, resp["ready_to_activate"])
	assert.NotEmpty(t, resp["wa_me_url"])
	assert.NotEmpty(t, resp["bot_number_display"])
}

func TestTokenStateHandler_200_NotReadyOmitsWaMeURL(t *testing.T) {
	stub := &stubGetTokenState{
		result: usecases.GetTokenStateResult{
			Output: output.GetTokenStateOutput{ReadyToActivate: false},
			Reason: usecases.TokenStateReasonNotFound,
		},
	}
	handler := buildStateHandler(stub)

	req := httptest.NewRequest(http.MethodGet, "/v1/onboarding/tokens/bad-token/state", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, false, resp["ready_to_activate"])
	assert.Nil(t, resp["wa_me_url"], "wa_me_url must be absent when not ready")
	assert.Nil(t, resp["bot_number_display"], "bot_number_display must be absent when not ready")
}

func TestTokenStateHandler_200_PendingStateDoesNotRevealReason(t *testing.T) {
	stub := &stubGetTokenState{
		result: usecases.GetTokenStateResult{
			Output: output.GetTokenStateOutput{ReadyToActivate: false},
			Reason: usecases.TokenStateReasonPending,
		},
	}
	handler := buildStateHandler(stub)

	req := httptest.NewRequest(http.MethodGet, "/v1/onboarding/tokens/tok-pending/state", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, false, resp["ready_to_activate"])
	assert.Nil(t, resp["wa_me_url"])
}

func TestTokenStateHandler_200_ExpiredStateDoesNotRevealReason(t *testing.T) {
	stub := &stubGetTokenState{
		result: usecases.GetTokenStateResult{
			Output: output.GetTokenStateOutput{ReadyToActivate: false},
			Reason: usecases.TokenStateReasonExpired,
		},
	}
	handler := buildStateHandler(stub)

	req := httptest.NewRequest(http.MethodGet, "/v1/onboarding/tokens/tok-expired/state", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, false, resp["ready_to_activate"])
}

func TestTokenStateHandler_200_ConsumedStateDoesNotRevealReason(t *testing.T) {
	stub := &stubGetTokenState{
		result: usecases.GetTokenStateResult{
			Output: output.GetTokenStateOutput{ReadyToActivate: false},
			Reason: usecases.TokenStateReasonConsumed,
		},
	}
	handler := buildStateHandler(stub)

	req := httptest.NewRequest(http.MethodGet, "/v1/onboarding/tokens/tok-consumed/state", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, false, resp["ready_to_activate"])
}

func TestTokenStateHandler_500_UseCaseError(t *testing.T) {
	stub := &stubGetTokenState{
		err: errors.New("database error"),
	}
	handler := buildStateHandler(stub)

	req := httptest.NewRequest(http.MethodGet, "/v1/onboarding/tokens/tok-err/state", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}
