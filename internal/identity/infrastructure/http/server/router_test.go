package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/http/server"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/http/server/handlers"
)

type stubUseCase struct{}

func (s *stubUseCase) Execute(_ context.Context, _ input.UpsertUserByWhatsApp) (output.UpsertUserByWhatsApp, error) {
	return output.UpsertUserByWhatsApp{ID: "stub-id"}, nil
}

func TestUserRouter_RegisterSemHandler_NaoPanica(t *testing.T) {
	rt := server.NewUserRouter(nil)
	r := chi.NewRouter()
	assert.NotPanics(t, func() {
		rt.Register(r)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/identity/users/", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestUserRouter_RegisterComHandler_RegistraEndpoint(t *testing.T) {
	stub := handlers.NewUpsertUserByWhatsAppHandler(&stubUseCase{}, noop.NewProvider())
	rt := server.NewUserRouter(stub)
	r := chi.NewRouter()
	rt.Register(r)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/identity/users/", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.NotEqual(t, http.StatusNotFound, rr.Code)
}
