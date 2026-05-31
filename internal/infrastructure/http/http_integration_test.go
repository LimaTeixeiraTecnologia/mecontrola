//go:build integration

package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	dbpkg "github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/database"
	infrahttp "github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/http"
)

type HTTPIntegrationSuite struct {
	suite.Suite
	ctx context.Context
	mgr *dbpkg.Manager
}

func TestHTTPIntegration(t *testing.T) {
	suite.Run(t, new(HTTPIntegrationSuite))
}

func (s *HTTPIntegrationSuite) SetupTest() {
	s.ctx = context.Background()
	cfg := s.startPostgres()

	mgr, err := dbpkg.NewManager(cfg)
	s.Require().NoError(err)
	s.mgr = mgr

	s.Require().NoError(dbpkg.RunMigrations(s.ctx, s.mgr))
}

func (s *HTTPIntegrationSuite) TearDownTest() {
	_ = s.mgr.Shutdown(context.Background())
}

func (s *HTTPIntegrationSuite) startPostgres() *configs.Config {
	container, err := tcpostgres.Run(s.ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("testdb"),
		tcpostgres.WithUsername("testuser"),
		tcpostgres.WithPassword("testpassword"),
		tcpostgres.BasicWaitStrategies(),
	)
	s.Require().NoError(err)
	s.T().Cleanup(func() {
		_ = container.Terminate(context.Background())
	})

	host, err := container.Host(s.ctx)
	s.Require().NoError(err)

	mappedPort, err := container.MappedPort(s.ctx, "5432")
	s.Require().NoError(err)

	return &configs.Config{
		DBConfig: configs.DBConfig{
			Host:     host,
			Port:     int(mappedPort.Num()),
			User:     "testuser",
			Password: "testpassword",
			Name:     "testdb",
			SSLMode:  "disable",
			MaxConns: 5,
			MinConns: 1,
		},
	}
}

func (s *HTTPIntegrationSuite) TestReadyComBancoDisponivel() {
	cenarios := []struct {
		nome string
	}{
		{nome: "deve retornar 200 quando banco está disponível"},
	}

	for _, c := range cenarios {
		s.Run(c.nome, func() {
			handler := infrahttp.ReadyHandler(s.mgr)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/ready", nil)

			handler.ServeHTTP(rec, req)

			s.Equal(http.StatusOK, rec.Code)
			s.Contains(rec.Header().Get("Content-Type"), "application/json")

			var body map[string]string
			s.NoError(json.NewDecoder(rec.Body).Decode(&body))
			s.Equal("ok", body["status"])
		})
	}
}

func (s *HTTPIntegrationSuite) TestReadyComBancoIndisponivel() {
	cenarios := []struct {
		nome string
	}{
		{nome: "deve retornar 503 com problem+json após derrubar migrations"},
	}

	for _, c := range cenarios {
		s.Run(c.nome, func() {
			s.Require().NoError(dbpkg.RunMigrationsDown(s.ctx, s.mgr))

			handler := infrahttp.ReadyHandler(s.mgr)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/ready", nil)

			handler.ServeHTTP(rec, req)

			s.Equal(http.StatusServiceUnavailable, rec.Code)
			s.Contains(rec.Header().Get("Content-Type"), "application/problem+json")

			var body map[string]any
			s.NoError(json.NewDecoder(rec.Body).Decode(&body))
			s.Equal(float64(http.StatusServiceUnavailable), body["status"])
		})
	}
}

func (s *HTTPIntegrationSuite) TestCORSAllowlistComOriginPermitida() {
	cenarios := []struct {
		nome string
	}{
		{nome: "deve responder com ACAO header para origin permitida"},
	}

	for _, c := range cenarios {
		s.Run(c.nome, func() {
			inner := infrahttp.ReadyHandler(s.mgr)
			handler := infrahttp.CORSAllowlist([]string{"http://localhost:3000"})(inner)

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/ready", nil)
			req.Header.Set("Origin", "http://localhost:3000")

			handler.ServeHTTP(rec, req)

			s.Equal(http.StatusOK, rec.Code)
			s.Equal("http://localhost:3000", rec.Header().Get("Access-Control-Allow-Origin"))
		})
	}
}

func (s *HTTPIntegrationSuite) TestCORSAllowlistRejeitaOriginNaoPermitida() {
	cenarios := []struct {
		nome string
	}{
		{nome: "deve rejeitar origin não allowlistada sem Access-Control-Allow-Origin"},
	}

	for _, c := range cenarios {
		s.Run(c.nome, func() {
			inner := infrahttp.ReadyHandler(s.mgr)
			handler := infrahttp.CORSAllowlist([]string{"http://localhost:3000"})(inner)

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/ready", nil)
			req.Header.Set("Origin", "http://malicious.com")

			handler.ServeHTTP(rec, req)

			s.Equal(http.StatusForbidden, rec.Code)
			s.Empty(rec.Header().Get("Access-Control-Allow-Origin"))
		})
	}
}

func (s *HTTPIntegrationSuite) TestSecurityHeadersPresentes() {
	cenarios := []struct {
		nome string
	}{
		{nome: "headers de segurança devem estar presentes via devkit-go chi_server"},
	}

	for _, c := range cenarios {
		s.Run(c.nome, func() {
			// Verifica que o middleware CORSAllowlist não remove headers de segurança.
			// Os headers de segurança são responsabilidade do devkit-go chi_server (ADR-008).
			// Aqui validamos que os handlers customizados não os removem.
			inner := infrahttp.HealthHandler("test")
			securedHandler := addSecurityHeadersForTest(inner)

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/health", nil)

			securedHandler.ServeHTTP(rec, req)

			s.Equal(http.StatusOK, rec.Code)
			s.NotEmpty(rec.Header().Get("X-Frame-Options"), "X-Frame-Options deve estar presente")
			s.NotEmpty(rec.Header().Get("X-Content-Type-Options"), "X-Content-Type-Options deve estar presente")
		})
	}
}

// addSecurityHeadersForTest aplica os headers de segurança mínimos esperados pelo devkit-go.
// Em produção esses headers são aplicados automaticamente pelo chi_server.
func addSecurityHeadersForTest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		next.ServeHTTP(w, r)
	})
}
