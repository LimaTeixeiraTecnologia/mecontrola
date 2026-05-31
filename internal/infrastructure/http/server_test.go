package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/database"
	infrahttp "github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/http"
)

// stubManager implementa apenas o necessário para testar sem conexão real.
type stubManager struct {
	healthErr error
}

func (m *stubManager) HealthCheck(_ context.Context) error {
	return m.healthErr
}

type ServerSuite struct {
	suite.Suite
	ctx context.Context
}

func TestServer(t *testing.T) {
	suite.Run(t, new(ServerSuite))
}

func (s *ServerSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *ServerSuite) TestHealthHandlerRetorna200Sempre() {
	cenarios := []struct {
		nome    string
		version string
	}{
		{"versão vazia retorna ok", ""},
		{"versão preenchida retorna ok", "abc123"},
	}

	for _, c := range cenarios {
		s.Run(c.nome, func() {
			handler := infrahttp.HealthHandler(c.version)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/health", nil)

			handler.ServeHTTP(rec, req)

			s.Equal(http.StatusOK, rec.Code)
			s.Contains(rec.Header().Get("Content-Type"), "application/json")

			var body map[string]string
			s.NoError(json.NewDecoder(rec.Body).Decode(&body))
			s.Equal("ok", body["status"])
		})
	}
}

func (s *ServerSuite) TestLiveHandlerRetorna200Sempre() {
	handler := infrahttp.LiveHandler("sha-test")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/live", nil)

	handler.ServeHTTP(rec, req)

	s.Equal(http.StatusOK, rec.Code)

	var body map[string]string
	s.NoError(json.NewDecoder(rec.Body).Decode(&body))
	s.Equal("ok", body["status"])
}

func (s *ServerSuite) TestReadyHandler() {
	cenarios := []struct {
		nome           string
		healthErr      error
		statusEsperado int
		contentType    string
	}{
		{
			nome:           "banco disponível retorna 200",
			healthErr:      nil,
			statusEsperado: http.StatusOK,
			contentType:    "application/json",
		},
		{
			nome:           "banco indisponível retorna 503 com problem+json",
			healthErr:      database.ErrConnection,
			statusEsperado: http.StatusServiceUnavailable,
			contentType:    "application/problem+json",
		},
		{
			nome:           "erro wrappado de conexão retorna 503",
			healthErr:      errors.Join(database.ErrConnection, errors.New("EOF")),
			statusEsperado: http.StatusServiceUnavailable,
			contentType:    "application/problem+json",
		},
	}

	for _, c := range cenarios {
		s.Run(c.nome, func() {
			mgr := &mockManagerHTTP{healthErr: c.healthErr}
			handler := infrahttp.ReadyHandlerFn(mgr.HealthCheck)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/ready", nil)

			handler.ServeHTTP(rec, req)

			s.Equal(c.statusEsperado, rec.Code)
			s.Contains(rec.Header().Get("Content-Type"), c.contentType)
		})
	}
}

func (s *ServerSuite) TestCORSAllowlistPermiteOriginExplicitada() {
	handler := infrahttp.CORSAllowlist([]string{"http://localhost:3000"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")

	handler.ServeHTTP(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	s.Equal("http://localhost:3000", rec.Header().Get("Access-Control-Allow-Origin"))
}

func (s *ServerSuite) TestCORSAllowlistRejeitaOriginNaoPermitida() {
	handler := infrahttp.CORSAllowlist([]string{"http://localhost:3000"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "http://malicious.com")

	handler.ServeHTTP(rec, req)

	s.Equal(http.StatusForbidden, rec.Code)
	s.Empty(rec.Header().Get("Access-Control-Allow-Origin"))
}

func (s *ServerSuite) TestCORSAllowlistListaVaziaRejeitaQualquerOrigin() {
	handler := infrahttp.CORSAllowlist([]string{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "http://qualquer.com")

	handler.ServeHTTP(rec, req)

	s.Equal(http.StatusForbidden, rec.Code)
}

func (s *ServerSuite) TestParseCORSOriginsIgnoraWildcard() {
	origins := infrahttp.ParseCORSOrigins("http://localhost:3000,*,http://localhost:5173")

	for _, o := range origins {
		s.NotEqual("*", o, "wildcard não deve estar presente na lista")
	}
}

// mockManagerHTTP é um stub local para testes de handler.
type mockManagerHTTP struct {
	healthErr error
}

func (m *mockManagerHTTP) HealthCheck(_ context.Context) error {
	return m.healthErr
}
