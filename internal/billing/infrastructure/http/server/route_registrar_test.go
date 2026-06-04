package server_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/suite"

	billinghttp "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/http/server"
)

type RouteRegistrarSuite struct {
	suite.Suite
}

func TestRouteRegistrar(t *testing.T) {
	suite.Run(t, new(RouteRegistrarSuite))
}

func (s *RouteRegistrarSuite) TestRegistrarRotaResponde() {
	stub := &stubIngestUseCase{}
	handler := billinghttp.NewKiwifyWebhookHandler(stub, slog.Default())
	registrar := billinghttp.NewKiwifyRouteRegistrar(handler)

	router := chi.NewRouter()
	registrar.Register(router)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/webhooks/kiwify", bytes.NewBufferString(`{}`))
	router.ServeHTTP(rec, req)

	s.NotEqual(http.StatusNotFound, rec.Code, "rota /webhooks/kiwify deve estar registrada")
}

func (s *RouteRegistrarSuite) TestRegistrarNilHandlerRetorna503() {
	registrar := billinghttp.NewKiwifyRouteRegistrar(nil)

	router := chi.NewRouter()
	registrar.Register(router)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/webhooks/kiwify", bytes.NewBufferString(`{}`))
	router.ServeHTTP(rec, req)

	s.Equal(http.StatusServiceUnavailable, rec.Code)
}

func (s *RouteRegistrarSuite) TestSetHandlerAtualiza() {
	registrar := billinghttp.NewKiwifyRouteRegistrar(nil)

	stub := &stubIngestUseCase{}
	handler := billinghttp.NewKiwifyWebhookHandler(stub, slog.Default())
	registrar.SetHandler(handler)

	router := chi.NewRouter()
	registrar.Register(router)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/webhooks/kiwify", bytes.NewBufferString(`{}`))
	router.ServeHTTP(rec, req)

	s.NotEqual(http.StatusServiceUnavailable, rec.Code)
}
