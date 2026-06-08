package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/http/server/handlers"
)

type stubCreateCheckout struct {
	result output.CreateCheckoutSessionOutput
	err    error
}

func (s *stubCreateCheckout) Execute(_ context.Context, in input.CreateCheckoutSessionInput) (output.CreateCheckoutSessionOutput, error) {
	if in.PlanID == "" {
		return output.CreateCheckoutSessionOutput{}, errors.New("plan_id is required")
	}
	return s.result, s.err
}

func buildCheckoutHandler(stub *stubCreateCheckout) http.Handler {
	o11y := noop.NewProvider()
	createdCount := 0
	rateLimitCount := 0
	h := handlers.NewCreateCheckoutHandler(
		stub,
		func(_ string) { createdCount++ },
		func() { rateLimitCount++ },
		o11y,
	)
	return http.HandlerFunc(h.Handle)
}

func TestCreateCheckoutHandler_201_Success(t *testing.T) {
	stub := &stubCreateCheckout{
		result: output.CreateCheckoutSessionOutput{
			CheckoutURL: "https://pay.kiwify.com.br/abc?sck=tok123",
			TokenID:     "token-id-1",
		},
	}
	handler := buildCheckoutHandler(stub)

	body := `{"plan_id":"11111111-1111-1111-1111-111111111111"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/onboarding/checkout", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "https://pay.kiwify.com.br/abc?sck=tok123", resp["checkout_url"])
}

func TestCreateCheckoutHandler_400_MissingPlanID(t *testing.T) {
	stub := &stubCreateCheckout{}
	handler := buildCheckoutHandler(stub)

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/v1/onboarding/checkout", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCreateCheckoutHandler_400_InvalidJSON(t *testing.T) {
	stub := &stubCreateCheckout{}
	handler := buildCheckoutHandler(stub)

	req := httptest.NewRequest(http.MethodPost, "/v1/onboarding/checkout", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCreateCheckoutHandler_400_UnknownPlan(t *testing.T) {
	stub := &stubCreateCheckout{err: application.ErrUnknownPlan}
	handler := buildCheckoutHandler(stub)

	body := `{"plan_id":"unknown-plan"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/onboarding/checkout", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCreateCheckoutHandler_503_CheckoutUnavailable(t *testing.T) {
	stub := &stubCreateCheckout{err: application.ErrCheckoutUnavailable}
	handler := buildCheckoutHandler(stub)

	body := `{"plan_id":"22222222-2222-2222-2222-222222222222"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/onboarding/checkout", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}

func TestCreateCheckoutHandler_CORS_AllowedOrigin(t *testing.T) {
	stub := &stubCreateCheckout{
		result: output.CreateCheckoutSessionOutput{CheckoutURL: "https://pay.kiwify.com.br/abc"},
	}
	o11y := noop.NewProvider()
	h := handlers.NewCreateCheckoutHandler(
		stub,
		func(_ string) {},
		func() {},
		o11y,
	)

	body := `{"plan_id":"11111111-1111-1111-1111-111111111111"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/onboarding/checkout", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://www.mecontrola.app.br")
	rr := httptest.NewRecorder()
	h.Handle(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
}
