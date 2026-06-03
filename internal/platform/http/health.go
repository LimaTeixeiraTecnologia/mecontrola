package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	infraerrors "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/errors"
)

// HealthResponse é o payload retornado pelos endpoints /health e /live.
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

// HealthHandler responde 200 sempre que o processo está vivo.
// GET /health.
func HealthHandler(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(HealthResponse{Status: "ok", Version: version})
	}
}

// LiveHandler implementa liveness puro: retorna 200 se o processo está em pé.
// GET /live.
func LiveHandler(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(HealthResponse{Status: "ok", Version: version})
	}
}

// HealthChecker define a interface mínima para o health check do banco.
type HealthChecker interface {
	HealthCheck(ctx context.Context) error
}

// ReadyHandler executa database.Manager.HealthCheck; responde 200 se OK, 503 com ProblemDetails se não.
// GET /ready.
func ReadyHandler(mgr *database.Manager) http.HandlerFunc {
	return ReadyHandlerFn(mgr.HealthCheck)
}

// ReadyHandlerFn permite injetar a função de health check diretamente (facilita testes unitários).
func ReadyHandlerFn(check func(ctx context.Context) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := check(r.Context()); err != nil {
			pd, status := infraerrors.ToProblemDetails(err)
			pd.Instance = r.URL.Path

			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(pd)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(HealthResponse{Status: "ok"})
	}
}
